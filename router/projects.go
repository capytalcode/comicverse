package router

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"forge.capytal.company/capytalcode/project-comicverse/service"
	"forge.capytal.company/capytalcode/project-comicverse/templates"
	"forge.capytal.company/loreddev/x/smalltrip/problem"
	"forge.capytal.company/loreddev/x/tinyssert"
	"github.com/google/uuid"
)

type projectController struct {
	projectSvc *service.Project

	templates templates.ITemplate

	assert tinyssert.Assertions
}

func newProjectController(
	projectService *service.Project,
	templates templates.ITemplate,
	assertions tinyssert.Assertions,
) *projectController {
	return &projectController{
		projectSvc: projectService,
		templates:  templates,
		assert:     assertions,
	}
}

func (ctrl projectController) dashboard(w http.ResponseWriter, r *http.Request) {
	userCtx := NewUserContext(r.Context())

	userID, ok := userCtx.GetUserID()
	if !ok {
		userCtx.Unathorize(w, r)
		return
	}

	projects, err := ctrl.projectSvc.GetUserProjects(userID)
	if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	ps := make([]struct {
		ID    string
		Title string
	}, len(projects))

	for i, project := range projects {
		ps[i] = struct {
			ID    string
			Title string
		}{
			ID:    base64.URLEncoding.EncodeToString([]byte(project.ID.String())),
			Title: project.Title,
		}
	}

	err = ctrl.templates.ExecuteTemplate(w, "dashboard", ps)
	if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
	}
}

func (ctrl projectController) getProject(w http.ResponseWriter, r *http.Request) {
	// TODO: Handle private projects

	shortProjectID := r.PathValue("projectID")

	id, err := base64.URLEncoding.DecodeString(shortProjectID)
	if err != nil {
		problem.NewBadRequest(fmt.Sprintf("Incorrectly encoded project ID: %s", err.Error())).ServeHTTP(w, r)
		return
	}

	projectID, err := uuid.ParseBytes(id)
	if err != nil {
		problem.NewBadRequest("Project ID is not a valid UUID").ServeHTTP(w, r)
		return
	}

	project, err := ctrl.projectSvc.GetProject(projectID)
	if errors.Is(err, service.ErrNotFound) {
		problem.NewNotFound().ServeHTTP(w, r)
		return
	} else if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	// TODO: Return project template
	b, err := json.Marshal(project)

	w.Header().Add("Content-Type", "application/json")
	if _, err := w.Write(b); err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}
}

func (ctrl projectController) createProject(w http.ResponseWriter, r *http.Request) {
	userCtx := NewUserContext(r.Context())

	userID, ok := userCtx.GetUserID()
	if !ok {
		userCtx.Unathorize(w, r)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		problem.NewBadRequest(`Missing "title" parameter`).ServeHTTP(w, r)
		return
	}

	project, err := ctrl.projectSvc.Create(title, userID)
	if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	path := fmt.Sprintf("/p/%s/", base64.URLEncoding.EncodeToString([]byte(project.ID.String())))
	http.Redirect(w, r, path, http.StatusSeeOther)
}
