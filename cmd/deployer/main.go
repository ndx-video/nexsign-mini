package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	defaultHosts = []string{
		"192.168.10.147",
		"192.168.10.174",
		"192.168.10.135",
		"192.168.10.211",
	}
)

type hostResult struct {
	host     string
	duration time.Duration
	err      error
}

func main() {
	var (
		hostsFlag    string
		keyFlag      string
		binaryFlag   string
		remoteDir    string
		parallelFlag int
		skipBuild    bool
	)

	homeDir, _ := os.UserHomeDir()

	flag.StringVar(&hostsFlag, "hosts", "all", "Comma-separated list of hosts or 'all'")
	flag.StringVar(&keyFlag, "key", filepath.Join(homeDir, ".ssh", "nsm-vbox.key"), "Path to SSH private key")
	flag.StringVar(&binaryFlag, "binary", "nsm", "Path for the compiled binary")
	flag.StringVar(&remoteDir, "remote-dir", "/home/nsm/nsm-app", "Remote deployment directory")
	flag.IntVar(&parallelFlag, "parallel", 2, "Number of hosts to deploy concurrently")
	flag.BoolVar(&skipBuild, "skip-build", false, "Skip rebuilding the binary before deployment")
	flag.Parse()

	hostList, err := resolveHosts(hostsFlag)
	if err != nil {
		log.Fatalf("resolve hosts: %v", err)
	}
	if len(hostList) == 0 {
		log.Fatal("no hosts specified")
	}
	if parallelFlag < 1 {
		parallelFlag = 1
	}
	if parallelFlag > len(hostList) {
		parallelFlag = len(hostList)
	}

	if err := ensureToolExists("rsync"); err != nil {
		log.Fatalf("rsync not available: %v", err)
	}
	if err := ensureToolExists("go"); err != nil {
		log.Fatalf("go toolchain not available: %v", err)
	}
	if err := ensureFileExists(keyFlag); err != nil {
		log.Fatalf("ssh key not accessible: %v", err)
	}

	binaryPath, err := filepath.Abs(binaryFlag)
	if err != nil {
		log.Fatalf("determine binary path: %v", err)
	}

	if !skipBuild {
		if err := generateDocs(); err != nil {
			log.Fatalf("generate docs: %v", err)
		}
		if err := buildBinary(binaryPath); err != nil {
			log.Fatalf("build binary: %v", err)
		}
	} else {
		log.Printf("Skipping build step (requested via --skip-build)")
	}

	results := runDeployments(hostList, keyFlag, binaryPath, remoteDir, parallelFlag)

	var failed int
	for _, r := range results {
		if r.err != nil {
			failed++
			log.Printf("[%s] ❌ deployment failed after %s: %v", r.host, r.duration.Truncate(time.Millisecond), r.err)
		} else {
			log.Printf("[%s] ✅ deployment completed in %s", r.host, r.duration.Truncate(time.Millisecond))
		}
	}

	if failed > 0 {
		log.Fatalf("deployment failed on %d host(s)", failed)
	}
}

func resolveHosts(flagValue string) ([]string, error) {
	if flagValue == "" || flagValue == "all" {
		return append([]string{}, defaultHosts...), nil
	}

	parts := strings.Split(flagValue, ",")
	var hosts []string
	for _, p := range parts {
		h := strings.TrimSpace(p)
		if h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts, nil
}

func ensureToolExists(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required tool %q not found in PATH", name)
	}
	return nil
}

func ensureFileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func buildBinary(binaryPath string) error {
	log.Printf("Building NSM binary -> %s", binaryPath)
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runDeployments(hosts []string, keyPath, binaryPath, remoteDir string, parallel int) []hostResult {
	var (
		wg       sync.WaitGroup
		sem      = make(chan struct{}, parallel)
		results  = make([]hostResult, len(hosts))
		rsyncDir = filepath.Join("internal", "web")
	)

	absDir, err := filepath.Abs(rsyncDir)
	if err != nil {
		log.Fatalf("resolve template directory: %v", err)
	}

	for idx, host := range hosts {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			err := deployHost(h, keyPath, binaryPath, absDir, remoteDir)
			results[i] = hostResult{
				host:     h,
				duration: time.Since(start),
				err:      err,
			}
		}(idx, host)
	}

	wg.Wait()
	return results
}

func deployHost(host, keyPath, binaryPath, webDir, remoteDir string) error {
	logPrefix := fmt.Sprintf("[%s]", host)
	log.Printf("%s Starting deployment", logPrefix)

	remoteUser := "nsm"
	sshTarget := fmt.Sprintf("%s@%s", remoteUser, host)

	// Ensure remote directory structure exists and stop existing binary.
	if err := stopRemoteBinary(sshTarget, keyPath); err != nil {
		return fmt.Errorf("stop remote binary: %w", err)
	}

	// Clean up database to force fresh start, but try to preserve identity
	cleanCmd := fmt.Sprintf("mkdir -p %[1]s/internal/web/static", remoteDir)
	if err := sshRun(sshTarget, keyPath, cleanCmd, 20*time.Second); err != nil {
		return fmt.Errorf("clean remote directories: %w", err)
	}

	// Push binary via rsync.
	if err := rsyncCopy(binaryPath, fmt.Sprintf("%s:%s/", sshTarget, remoteDir), keyPath); err != nil {
		return fmt.Errorf("rsync binary: %w", err)
	}

	// Push templates and static assets.
	if err := rsyncCopy(webDir+"/", fmt.Sprintf("%s:%s/internal/web/", sshTarget, remoteDir), keyPath); err != nil {
		return fmt.Errorf("rsync templates: %w", err)
	}

	if err := sshRun(sshTarget, keyPath, fmt.Sprintf("chmod +x %s/nsm", remoteDir), 5*time.Second); err != nil {
		return fmt.Errorf("set executable bit: %w", err)
	}

	startCmd := fmt.Sprintf("cd %s && setsid -f nohup ./nsm > nsm.log 2>&1 < /dev/null", remoteDir)
	if err := sshRun(sshTarget, keyPath, startCmd, 30*time.Second); err != nil {
		return fmt.Errorf("start remote binary: %w", err)
	}

	// Give the process a moment to start, then verify.
	time.Sleep(2 * time.Second)
	if err := sshRun(sshTarget, keyPath, "pgrep -f 'nsm$'", 5*time.Second); err != nil {
		// Fetch log to debug startup failure
		log.Printf("%s Process failed to start. Fetching nsm.log...", logPrefix)
		logCmd := fmt.Sprintf("cat %s/nsm.log", remoteDir)
		if logErr := sshRun(sshTarget, keyPath, logCmd, 5*time.Second); logErr != nil {
			log.Printf("%s Failed to fetch log: %v", logPrefix, logErr)
		}
		return fmt.Errorf("verify process running: %w", err)
	}

	log.Printf("%s Deployment succeeded", logPrefix)
	return nil
}

func sshRun(target, keyPath, remoteCmd string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"-i", keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		target,
		remoteCmd,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("ssh command timed out: %s", remoteCmd)
		}
		return fmt.Errorf("ssh error (%s): %v | output: %s", remoteCmd, err, strings.TrimSpace(output.String()))
	}
	if out := strings.TrimSpace(output.String()); out != "" {
		log.Printf("[%s] %s", target, out)
	}
	return nil
}

func rsyncCopy(src, dest, keyPath string) error {
	args := []string{
		"-az",
		"--delete",
		"--exclude=identity.id",
		"--exclude=hosts.db",
		"--exclude=hosts.json",
		"-e", fmt.Sprintf("ssh -i %s -o BatchMode=yes -o StrictHostKeyChecking=no", keyPath),
		src,
		dest,
	}

	cmd := exec.Command("rsync", args...)
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync output: %s | err: %w", strings.TrimSpace(output.String()), err)
	}

	if out := strings.TrimSpace(output.String()); out != "" {
		log.Printf("[rsync] %s", out)
	}
	return nil
}

func stopRemoteBinary(target, keyPath string) error {
	stopCmd := "pgrep -f 'nsm$' >/dev/null && pkill -TERM 'nsm$' || true"
	if err := sshRun(target, keyPath, stopCmd, 15*time.Second); err != nil {
		return err
	}

	waitCmd := "count=0; while pgrep -f 'nsm$' >/dev/null; do if [ \"$count\" -ge 15 ]; then exit 1; fi; count=$((count+1)); sleep 1; done"
	return sshRun(target, keyPath, waitCmd, 20*time.Second)
}

func generateDocs() error {
	log.Println("Generating API documentation...")
	cmd := exec.Command("go", "run", "cmd/docgen/main.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

