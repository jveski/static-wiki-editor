package main

import (
	"embed"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

//go:embed assets
var assetFS embed.FS

var editorTempl = template.Must(template.New("").Parse(`
<link href="/assets/quill.snow.css" rel="stylesheet" />
<script src="/assets/quill.js"></script>

<form method="post">
{{- if .modified -}}
    <div id="updated-banner">
    Update was successful, but may take a few minutes to be applied.
    </div>
{{- end -}}

    <div id="editor">{{ .content }}</div>
    <button id="save" type="submit">Save Changes</button>
</form>

<style>
    body {
        font-family: sans-serif;
    }

    #editor {
        height: 60%;
    }

    #save {
        border: 1px solid #000;
        padding: 6px;
        border-radius: 3px;
        background: transparent;
        margin-top: 10px;
        font-size: 100%;
        cursor: pointer;
    }

    #updated-banner {
        padding: 15px;
        background: #fffec1;
        margin: 15px;
    }
</style>

<script>
    const quill = new Quill('#editor', {
        theme: 'snow',
        modules: {
            toolbar: [
                [{ header: [1, 2, false] }],
                ['bold', 'italic', 'underline'],
                ['image'],
            ],
        },
    })

    const form = document.querySelector('form')
    const editor = document.getElementById("editor")
    form.addEventListener('formdata', (event) => {
        event.formData.append('content', editor.children[0].innerHTML)
    })
</script>
`))

func main() {
	router := http.NewServeMux()
	assets := http.FileServer(http.FS(assetFS))

	var (
		syncInterval   = flag.Duration("sync-interval", time.Minute*5, "How often to sync git repo (not including actions caused by incoming requests)")
		syncCooldown   = flag.Duration("sync-cooldown", time.Second*10, "Min interval between git pushes")
		allowAnonymous = flag.Bool("allow-anonymous", false, "(insecure!) Allow anyone to edit. If false, X-Forwarded-Email is used to authenticate users")
		remote         = flag.String("remote", "", "Git remote used when bootstrapping the local state")
	)
	flag.Parse()

	err := initializeRepo(*remote)
	if err != nil {
		panic(err)
	}

	// Sync asynchronously with the remote
	notify := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(*syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-notify:
			}

			start := time.Now()
			slog.Info("syncing with remote...")

			err := pushPull()
			if err != nil {
				slog.Error("error while syncing remote repository", "error", err)
				continue
			}

			slog.Info("synced with remote", "latencyMS", time.Since(start).Milliseconds())
			time.Sleep(*syncCooldown)
		}
	}()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { assets.ServeHTTP(w, r) })
	router.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	router.HandleFunc("/edit/", func(w http.ResponseWriter, r *http.Request) {
		page := strings.TrimPrefix(r.URL.Path, "/edit/")

		// Authenticate the user
		email := r.Header.Get("X-Forwarded-Email")
		if email == "" && !*allowAnonymous {
			http.Error(w, "unauthenticated!", 401)
			return
		}
		if email == "" {
			email = "<anonymous>"
		}

		// Handle form submission
		if r.Method == http.MethodPost {
			slog.Info("staging page update", "page", page)

			content := r.PostFormValue("content")
			err := stageUpdate(page, content, email)
			if err != nil {
				slog.Error("error while staging page update", "error", err)
				http.Error(w, "system error", 500)
				return
			}

			select {
			case notify <- struct{}{}: // schedule sync unless already scheduled
			default:
			}
		}

		// Read the current page contents
		slog.Info("reading page", "page", page)
		pageHTML, found, err := readPage(page)
		if err != nil {
			slog.Error("unable to read page", "error", err)
			http.Error(w, "system error", 500)
			return
		}
		if !found {
			slog.Warn("page was not found", "page", page)
			http.Error(w, "The requested page was not found", 404)
			return
		}

		// Render the editor page
		w.Header().Set("Content-Type", "text/html")
		err = editorTempl.Execute(w, map[string]any{
			"content":  pageHTML,
			"modified": r.Method == http.MethodPost,
		})
		if err != nil {
			slog.Error("unable to render template", "error", err)
		}
	})

	panic(http.ListenAndServe(":8080", router))
}

var gitLock sync.Mutex

func git(args ...string) error {
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w - stderr: %s", err, out)
	}
	return nil
}

func initializeRepo(remote string) error {
	gitLock.Lock()
	defer gitLock.Unlock()

	if _, err := os.Stat(".git"); err != nil {
		err := git("init")
		if err != nil {
			return fmt.Errorf("initializing: %w", err)
		}

		err = git("remote", "add", "origin", remote)
		if err != nil {
			return fmt.Errorf("adding remote: %w", err)
		}
	}

	// Discard any partially applied writes that may have been left around by a previous (crashed) process
	err := git("reset", "--hard")
	if err != nil {
		return fmt.Errorf("resetting: %w", err)
	}

	err = git("fetch", "origin", "main")
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}

	err = git("checkout", "main")
	if err != nil {
		return fmt.Errorf("checking out: %w", err)
	}

	return nil
}

func pushPull() error {
	gitLock.Lock()
	defer gitLock.Unlock()

	err := git("pull", "--rebase", "origin", "main")
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}

	err = git("push", "origin", "main")
	if err != nil {
		return fmt.Errorf("pushing: %w", err)
	}

	return nil
}

func readPage(page string) (string, bool, error) {
	gitLock.Lock()
	defer gitLock.Unlock()

	raw, err := os.ReadFile(filepath.Join("content", page) + ".md")
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading file: %w", err)
	}

	return mdToHTML(raw), true, nil
}

func stageUpdate(page, html, email string) error {
	gitLock.Lock()
	defer gitLock.Unlock()

	md, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		return err
	}

	path := filepath.Join("content", page) + ".md"
	_, err = os.Stat(path) // don't let editors create new files
	if err != nil {
		return fmt.Errorf("reading existing file: %w", err)
	}

	err = os.WriteFile(path, []byte(md), 0644)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	err = git("add", path)
	if err != nil {
		return fmt.Errorf("adding file: %w", err)
	}

	return git("commit", "--allow-empty", "-m", fmt.Sprintf("Update %s\nAuthored by: %s\n", page, email))
}

func mdToHTML(md []byte) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer))
}