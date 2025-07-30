package router

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"forge.capytal.company/capytalcode/project-comicverse/service"
	"forge.capytal.company/capytalcode/project-comicverse/templates"
	"forge.capytal.company/loreddev/x/smalltrip"
	"forge.capytal.company/loreddev/x/smalltrip/middleware"
	"forge.capytal.company/loreddev/x/smalltrip/problem"
	"forge.capytal.company/loreddev/x/tinyssert"
)

type router struct {
	userService    *service.User
	tokenService   *service.Token
	projectService *service.Project

	templates templates.ITemplate
	assets    fs.FS
	cache     bool

	assert tinyssert.Assertions
	log    *slog.Logger
}

func New(cfg Config) (http.Handler, error) {
	if cfg.UserService == nil {
		return nil, errors.New("user service is nil")
	}
	if cfg.TokenService == nil {
		return nil, errors.New("token service is nil")
	}
	if cfg.ProjectService == nil {
		return nil, errors.New("project service is nil")
	}
	if cfg.Templates == nil {
		return nil, errors.New("templates is nil")
	}
	if cfg.Assets == nil {
		return nil, errors.New("static files is nil")
	}
	if cfg.Assertions == nil {
		return nil, errors.New("assertions is nil")
	}
	if cfg.Logger == nil {
		return nil, errors.New("logger is nil")
	}

	r := &router{
		userService:    cfg.UserService,
		tokenService:   cfg.TokenService,
		projectService: cfg.ProjectService,

		templates: cfg.Templates,
		assets:    cfg.Assets,
		cache:     !cfg.DisableCache,

		assert: cfg.Assertions,
		log:    cfg.Logger,
	}

	return r.setup(), nil
}

type Config struct {
	UserService    *service.User
	TokenService   *service.Token
	ProjectService *service.Project

	Templates    templates.ITemplate
	Assets       fs.FS
	DisableCache bool

	Assertions tinyssert.Assertions
	Logger     *slog.Logger
}

func (router *router) setup() http.Handler {
	router.assert.NotNil(router.log)
	router.assert.NotNil(router.assets)

	log := router.log

	log.Debug("Initializing router")

	r := smalltrip.NewRouter(
		smalltrip.WithAssertions(router.assert),
		smalltrip.WithLogger(log.WithGroup("smalltrip")),
	)

	r.Use(middleware.Logger(log.WithGroup("requests")))
	if router.cache {
		r.Use(middleware.Cache())
	} else {
		r.Use(middleware.DisableCache())
	}

	r.Use(problem.PanicMiddleware())
	// TODO: when the HandlerDevpage is completed on the problem package, we
	// will provide it a custom template here:
	// r.Use(problem.Middleware())

	userController := newUserController(userControllerCfg{
		UserService:  router.userService,
		TokenService: router.tokenService,
		LoginPath:    "/login/",
		RedirectPath: "/",
		Templates:    router.templates,
		Assert:       router.assert,
	})
	projectController := newProjectController(router.projectService, router.templates, router.assert)

	r.Handle("/assets/", http.StripPrefix("/assets/", http.FileServerFS(router.assets)))

	r.Use(userController.userMiddleware)

	r.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add a way to the user to bypass this check and see the landing page.
		//       Probably a query parameter to bypass like "?landing=true"
		if _, ok := NewUserContext(r.Context()).GetUserID(); ok {
			projectController.dashboard(w, r)
			return
		}

		err := router.templates.ExecuteTemplate(w, "landing", nil)
		if err != nil {
			problem.NewInternalServerError(err).ServeHTTP(w, r)
		}
	})

	r.HandleFunc("/login/{$}", userController.login)
	r.HandleFunc("/register/{$}", userController.register)

	// TODO: Provide/redirect short project-id paths to long paths with the project title as URL /projects/title-of-the-project-<start of uuid>
	r.HandleFunc("GET /p/{projectID}/{$}", projectController.getProject)
	r.HandleFunc("POST /p/{$}", projectController.createProject)

	return r
}

// getMethod is a helper function to get the HTTP method of request, tacking precedence
// the "x-method" argument sent by requests via form or query values.
func getMethod(r *http.Request) string {
	m := r.FormValue("x-method")
	if m != "" {
		return strings.ToUpper(m)
	}

	return strings.ToUpper(r.Method)
}
