package httpserver

import (
	"html/template"
	"log/slog"
	"net/http"

	"yadro.com/course/web/core"
)

type Handler struct {
	service    *core.WebService
	tmpls      *template.Template
	staticPath string
}

func NewHandler(s *core.WebService, tmplPath string, staticPath string) *Handler {
	// Предварительная загрузка шаблонов[cite: 6]
	t := template.Must(template.ParseGlob(tmplPath))
	return &Handler{
		service:    s,
		tmpls:      t,
		staticPath: staticPath,
	}
}

// Mux настраивает маршруты для веб-интерфейса
func (h *Handler) Mux() *http.ServeMux {
	mux := http.NewServeMux()

	// Раздача статики (CSS, JS)
	fileServer := http.FileServer(http.Dir(h.staticPath))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Основные страницы
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/search", h.handleSearch)

	return mux
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, "index.html", core.PageData{})
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	comics, err := h.service.SearchComics(query)
	if err != nil {
		slog.Error("search failed", "query", query, "error", err)
		h.render(w, "index.html", core.PageData{
			Query: query,
			Error: "Something went wrong. Please try again later.",
		})
		return
	}

	h.render(w, "index.html", core.PageData{
		Comics: comics,
		Query:  query,
	})
}

func (h *Handler) render(w http.ResponseWriter, name string, data core.PageData) {
	if err := h.tmpls.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template rendering failed", "name", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
