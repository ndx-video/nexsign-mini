TESTING.md
===========

This file documents how to run the `test-deploy.sh` deployment helper and how to
monitor it during execution. Keep this file in the repository so other
contributors can follow the same workflow.

Overview
--------

`test-deploy.sh` builds the `nsm` binary, optionally uploads a hosts file to
targets (bootstrap), copies the binary to each target, optionally installs a
systemd unit, and runs light smoke checks (HTTP `/ping` and MCP `/rpc`)
(when requested).

Automatic logging
-----------------

The script now writes a timestamped logfile automatically to the `logs/`
directory. On each run it creates a file named like:

  logs/deploy-YYYYMMDD-HHMMSS.log

All stdout and stderr produced by the script is streamed to the console and
appended to that logfile (via `tee`). You do not need to pipe the script
externally to `tee` anymore.

Quick examples
--------------

Dry-run (no changes, just prints actions):

```bash
./test-deploy.sh --dry-run --verbose --upload-hosts /etc/hosts
```

Real deploy (uploads local `/etc/hosts`, runs smoke checks, 2 hosts in
parallel):

```bash
./test-deploy.sh --verbose --smoke --mcp-check --parallel 2 --upload-hosts /etc/hosts
```

Monitoring the logfile in real time
----------------------------------

In another terminal you can follow progress with:

```bash
# Replace the glob if you want a specific logfile
tail -f logs/deploy-*.log

# or watch the tail output every second
watch -n 1 'tail -n 120 logs/deploy-*.log'
```

If you want to see only the most recent logfile once it's created:

```bash
ls -1t logs/deploy-*.log | head -n1 | xargs tail -f
```

Notes & cautions
----------------

- The `--upload-hosts` option will replace `/etc/hosts` on the target(s).
  The script backs up the original to `/etc/hosts.bak.<timestamp>` before
  replacing it.
- Replacing `/etc/hosts` can break name resolution if the provided file is
  missing required entries. For long-term operation prefer a bootstrap-only
  approach or a secure distributed registry (ledger) of host identities.
- The script uses `sudo` for privileged operations on the remote. Ensure the
  `SSH_USER` has passwordless sudo for the needed operations or be prepared to
  run the script interactively so you can enter a password if prompted.

Troubleshooting
---------------

- If the script fails with "unable to resolve host ..." on the remote, use
  `--upload-hosts` to ensure the remote `/etc/hosts` contains the correct
  mapping for the node's hostname during bootstrap.
- Check the logfile in `logs/` for detailed output; the script also prints
  progress to the console.

Contact
-------

If you need help with the deploy workflow, leave a note in the repository's
issue tracker or ping the team lead.
