package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Endpoint struct {
	Title       string
	Route       string
	Description string
	Response    string
}

func main() {
	apiDir := "internal/api"
	files, err := os.ReadDir(apiDir)
	if err != nil {
		panic(err)
	}

	var endpoints []Endpoint

	// Regex to match comments
	reTitle := regexp.MustCompile(`// @Title: (.*)`)
	reRoute := regexp.MustCompile(`// @Route: (.*)`)
	reDesc := regexp.MustCompile(`// @Description: (.*)`)
	reResp := regexp.MustCompile(`// @Response: (.*)`)

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		f, err := os.Open(filepath.Join(apiDir, file.Name()))
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		var current Endpoint
		
		for scanner.Scan() {
			line := scanner.Text()
			
			if match := reTitle.FindStringSubmatch(line); len(match) > 1 {
				current.Title = strings.TrimSpace(match[1])
			}
			if match := reRoute.FindStringSubmatch(line); len(match) > 1 {
				current.Route = strings.TrimSpace(match[1])
			}
			if match := reDesc.FindStringSubmatch(line); len(match) > 1 {
				current.Description = strings.TrimSpace(match[1])
			}
			if match := reResp.FindStringSubmatch(line); len(match) > 1 {
				current.Response = strings.TrimSpace(match[1])
				// End of block, append and reset
				if current.Title != "" && current.Route != "" {
					endpoints = append(endpoints, current)
					current = Endpoint{}
				}
			}
		}
	}

	generateHTML(endpoints)
}

func generateHTML(endpoints []Endpoint) {
	html := `
<div class="flex h-full gap-6">
  <!-- Main Content: Endpoints List -->
  <div class="flex-1 min-w-0">
    <div class="my-2 text-center">
      <div class="text-sm font-semibold text-desert-fg">API Reference</div>
      <div class="text-sm text-desert-tan">Auto-generated from code comments</div>
    </div>

    <div class="space-y-4">
      <div class="rounded p-4 border border-desert-gray">
        <h3 class="font-medium mb-3 text-desert-yellow">Endpoints</h3>
        <div class="space-y-3 text-sm font-mono">
`

	for _, ep := range endpoints {
		method := strings.Split(ep.Route, " ")[0]
		color := "desert-cyan"
		if method == "POST" { color = "desert-green" }
		if method == "DELETE" { color = "desert-red" }
		
		// Extract path and params
		fullPath := strings.TrimPrefix(ep.Route, method+" ")
		parts := strings.Split(fullPath, "?")
		path := parts[0]
		params := ""
		if len(parts) > 1 {
			params = parts[1]
		}

		// Escape for JS string
		jsRoute := strings.ReplaceAll(ep.Route, "\"", "\\\"")
		jsDesc := strings.ReplaceAll(ep.Description, "\"", "\\\"")
		jsMethod := method
		jsPath := path
		jsParams := params

		html += fmt.Sprintf(`
          <div class="border-l-2 border-%s pl-3 cursor-pointer hover:bg-desert-darkgray transition-colors p-2 rounded"
               onclick="selectEndpoint('%s', '%s', '%s', '%s', '%s')">
            <div class="text-%s font-bold">%s</div>
            <div class="text-desert-tan text-xs mt-1">%s</div>
            <div class="text-desert-tan text-xs mt-1">Response: %s</div>
          </div>`, 
          color, 
          jsMethod, jsPath, jsParams, jsDesc, jsRoute,
          color, ep.Route, ep.Description, ep.Response)
	}

	html += `
        </div>
      </div>
    </div>
  </div>

  <!-- Sidebar: Interactive Console -->
  <div class="w-80 flex-none hidden md:block">
    <div class="sticky top-4 bg-desert-darkgray rounded shadow-lg p-4 border border-desert-gray">
      <h3 class="font-medium mb-3 text-desert-yellow">Try It Out</h3>
      
      <div id="console-empty" class="text-desert-gray text-sm italic text-center py-8">
        Select an endpoint to test
      </div>

      <div id="console-form" class="hidden space-y-4">
        <div>
          <div class="text-xs text-desert-tan font-mono mb-1">Endpoint</div>
          <div id="console-route" class="text-sm font-bold text-desert-fg break-all"></div>
          <div id="console-desc" class="text-xs text-desert-gray mt-1"></div>
        </div>

        <form id="api-form" onsubmit="submitRequest(event)" target="_blank" class="space-y-3">
          <input type="hidden" id="method" name="_method">
          <input type="hidden" id="path" name="_path">

          <div id="params-container" class="space-y-2">
            <!-- Params injected here -->
          </div>

          <div id="body-container" class="hidden">
            <label class="block text-xs text-desert-tan mb-1">Request Body (JSON)</label>
            <textarea id="json-body" class="w-full h-32 bg-desert-bg text-desert-fg text-xs font-mono p-2 rounded border border-desert-gray focus:border-desert-yellow outline-none" placeholder="{}"></textarea>
          </div>

          <button type="submit" class="w-full bg-desert-yellow text-desert-bg font-bold py-2 px-4 rounded hover:bg-desert-orange transition-colors text-sm">
            Send Request
          </button>
        </form>
      </div>
    </div>
  </div>
</div>

`
	
	os.WriteFile("internal/web/api-view.html", []byte(html), 0644)
	fmt.Println("Generated internal/web/api-view.html")
}
