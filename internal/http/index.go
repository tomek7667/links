package http

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/tomek7667/links/internal/domain"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Links</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #1e1e1e;
            color: #e0e0e0;
            padding: 24px;
        }
        .container { max-width: 900px; margin: 0 auto; }
        .add-form {
            display: flex;
            gap: 10px;
            margin-bottom: 24px;
            flex-wrap: wrap;
        }
        .add-form input {
            padding: 14px 16px;
            font-size: 16px;
            border: 1px solid #444;
            border-radius: 4px;
            flex: 1;
            min-width: 160px;
            background: #2d2d2d;
            color: #e0e0e0;
        }
        .add-form input:focus { outline: none; border-color: #888; }
        .add-form button {
            padding: 14px 24px;
            font-size: 16px;
            background: transparent;
            color: #e0e0e0;
            border: 1px solid #444;
            border-radius: 4px;
            cursor: pointer;
        }
        .add-form button:hover { border-color: #888; }
        .links-list { list-style: none; }
        .link-item {
            display: flex;
            align-items: center;
            background: #2d2d2d;
            margin-bottom: 8px;
            border-radius: 4px;
            border: 1px solid #3a3a3a;
        }
        .link-item:hover { background: #353535; }
        .link-item a {
            flex: 1;
            padding: 18px 20px;
            font-size: 17px;
            color: #e0e0e0;
            text-decoration: none;
        }
        .link-url { color: #888; font-size: 14px; margin-left: 10px; }
        .delete-btn {
            padding: 18px 20px;
            font-size: 14px;
            background: transparent;
            color: #888;
            border: none;
            border-left: 1px solid #3a3a3a;
            cursor: pointer;
        }
        .delete-btn:hover { background: #4a2a2a; color: #e57373; }
        .empty { color: #888; padding: 24px; text-align: center; }
    </style>
</head>
<body>
    <div class="container">
        <form class="add-form" id="addForm">
            <input type="text" id="title" placeholder="Title" required>
            <input type="url" id="url" placeholder="https://example.com" required>
            <button type="submit">Add</button>
        </form>
        <ul class="links-list" id="linksList">
            {{range .}}
            <li class="link-item">
                <a href="{{.Url}}" target="_blank">{{.Title}}<span class="link-url">({{.Url}})</span></a>
                <button class="delete-btn" onclick="deleteLink('{{.Url}}')">Delete</button>
            </li>
            {{else}}
            <li class="empty">No links yet</li>
            {{end}}
        </ul>
    </div>
    <script>
        document.getElementById('addForm').onsubmit = async (e) => {
            e.preventDefault();
            const title = document.getElementById('title').value;
            const url = document.getElementById('url').value;
            await fetch('/api/links', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({title, url})
            });
            location.reload();
        };
        async function deleteLink(url) {
            await fetch('/api/links', {
                method: 'DELETE',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({url})
            });
            location.reload();
        }
    </script>
</body>
</html>`

var indexTmpl = template.Must(template.New("index").Parse(indexHTML))

func (s *Server) AddIndexRoute() {
	s.r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		links := s.dber.GetLinks()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		indexTmpl.Execute(w, links)
	})

	s.r.Post("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var link domain.Link
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.SaveLink(link)
		w.WriteHeader(http.StatusCreated)
	})

	s.r.Delete("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Url string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.DeleteLink(req.Url)
		w.WriteHeader(http.StatusOK)
	})
}
