package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

func orgRepo(repo string) string {
	or := strings.TrimPrefix(repo, "https://")
	or = strings.TrimPrefix(or, "http://")
	or = strings.TrimPrefix(or, "git@")
	or = strings.TrimPrefix(or, "github.com/")
	or = strings.TrimSuffix(or, ".git")
	or = strings.TrimSuffix(or, "/")
	return or
}

func rawURL(repo string, paths ...string) string {
	return "https://raw.githubusercontent.com/" + orgRepo(repo) + "/refs/heads/main/" + filepath.Join(paths...)
}

func downloadRaw(repo string, paths ...string) ([]byte, error) {
	resp, err := http.Get(rawURL(repo, paths...))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: %s", resp.Status)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bb, nil
}

func validateDiskSize(size string) error {
	if size == "" {
		return fmt.Errorf("disk size cannot be empty")
	}
	unitValid := false
	diskSize := ""
	validUnits := []string{"MB", "MiB", "GB", "GiB", "TB", "TiB"}
	for _, u := range validUnits {
		if strings.HasSuffix(size, u) {
			unitValid = true
			diskSize = size[:len(size)-len(u)]
			break
		}
	}
	if !unitValid {
		return fmt.Errorf("size must have a valid unit (MB, MiB, GB, GiB, TB, TiB)")
	}

	//
	// diskSize must be a valid number
	// diskSize must be greater than 0

	iDiskSize, err := strconv.Atoi(diskSize)
	if err != nil {
		return fmt.Errorf("size must be a valid number")
	}
	if iDiskSize <= 0 {
		return fmt.Errorf("size must be greater than 0")
	}

	return nil
}

func WelcomeMessage(app Application, launch LaunchSettings) (string, error) {
	t1 := template.New("welcome")
	t1, err := t1.Parse(welcomeMessage)
	if err != nil {
		panic(err)
	}

	bb := bytes.Buffer{}
	err = t1.Execute(&bb, map[string]interface{}{
		"app":    app,
		"launch": launch,
	})
	return bb.String(), err
}

var welcomeMessage = `# Instance Details

- Instance Name: {{ .launch.Name }}
- Application: {{.app.Name}}
- Image: {{.launch.Image}}
- Incus Profiles: {{range .launch.Profiles}}{{.}} {{end}}
{{if .app.DefaultCredentials.Username }}- Default Credentials: User:{{.app.DefaultCredentials.Username}} / Password: {{.app.DefaultCredentials.Password}}{{end}}

## Application Information
{{.app.Description}}

## Resources
Website: [{{.app.Name}}]({{.app.Website}})

Documentation: [{{.app.Name}}]({{.app.Documentation}})

{{if ne .app.InterfacePort 0 }}Application Port : {{.app.InterfacePort}}{{end}}

`
