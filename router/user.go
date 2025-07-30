package router

import (
	"context"
	"errors"
	"net/http"

	"forge.capytal.company/capytalcode/project-comicverse/service"
	"forge.capytal.company/capytalcode/project-comicverse/templates"
	"forge.capytal.company/loreddev/x/smalltrip/middleware"
	"forge.capytal.company/loreddev/x/smalltrip/problem"
	"forge.capytal.company/loreddev/x/tinyssert"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type userController struct {
	userSvc  *service.User
	tokenSvc *service.Token

	loginPath    string
	redirectPath string
	templates    templates.ITemplate

	assert tinyssert.Assertions
}

func newUserController(cfg userControllerCfg) userController {
	cfg.Assert.NotNil(cfg.UserService)
	cfg.Assert.NotNil(cfg.TokenService)
	cfg.Assert.NotZero(cfg.LoginPath)
	cfg.Assert.NotZero(cfg.RedirectPath)
	cfg.Assert.NotNil(cfg.Templates)

	return userController{
		userSvc:      cfg.UserService,
		tokenSvc:     cfg.TokenService,
		loginPath:    cfg.LoginPath,
		redirectPath: cfg.RedirectPath,
		templates:    cfg.Templates,
		assert:       cfg.Assert,
	}
}

type userControllerCfg struct {
	UserService  *service.User
	TokenService *service.Token

	LoginPath    string
	RedirectPath string
	Templates    templates.ITemplate

	Assert tinyssert.Assertions
}

func (ctrl userController) login(w http.ResponseWriter, r *http.Request) {
	ctrl.assert.NotNil(ctrl.templates) // TODO?: Remove these types of assertions, since golang will panic anyway
	ctrl.assert.NotNil(ctrl.userSvc)   //        when the methods of these functions are called

	if r.Method == http.MethodGet {
		err := ctrl.templates.ExecuteTemplate(w, "login", nil)
		if err != nil {
			problem.NewInternalServerError(err).ServeHTTP(w, r)
		}
		return
	}
	if r.Method != http.MethodPost {
		problem.NewMethodNotAllowed([]string{http.MethodGet, http.MethodPost}).ServeHTTP(w, r)
		return
	}

	username, passwd := r.FormValue("username"), r.FormValue("password")
	if username == "" {
		problem.NewBadRequest(`Missing "username" form value`).ServeHTTP(w, r)
		return
	}
	if passwd == "" {
		problem.NewBadRequest(`Missing "password" form value`).ServeHTTP(w, r)
		return
	}

	// TODO: Move token issuing to it's own service, make UserService.Login just return the user
	user, err := ctrl.userSvc.Login(username, passwd)
	if errors.Is(err, service.ErrNotFound) {
		problem.NewNotFound().ServeHTTP(w, r)
		return
	} else if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	token, err := ctrl.tokenSvc.Issue(user)
	if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	// TODO: harden the cookie policy to the same domain
	cookie := &http.Cookie{
		Path:     "/",
		HttpOnly: true,
		Name:     "authorization",
		Value:    token,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, ctrl.redirectPath, http.StatusSeeOther)
}

func (ctrl userController) register(w http.ResponseWriter, r *http.Request) {
	ctrl.assert.NotNil(ctrl.templates)
	ctrl.assert.NotNil(ctrl.userSvc)

	if r.Method == http.MethodGet {
		err := ctrl.templates.ExecuteTemplate(w, "register", nil)
		if err != nil {
			problem.NewInternalServerError(err).ServeHTTP(w, r)
		}
		return
	}

	if r.Method != http.MethodPost {
		problem.NewMethodNotAllowed([]string{http.MethodGet, http.MethodPost}).ServeHTTP(w, r)
		return
	}

	username, passwd := r.FormValue("username"), r.FormValue("password")
	if username == "" {
		problem.NewBadRequest(`Missing "username" form value`).ServeHTTP(w, r)
		return
	}
	if passwd == "" {
		problem.NewBadRequest(`Missing "password" form value`).ServeHTTP(w, r)
		return
	}

	user, err := ctrl.userSvc.Register(username, passwd)
	if errors.Is(err, service.ErrUsernameAlreadyExists) || errors.Is(err, service.ErrPasswordTooLong) {
		problem.NewBadRequest(err.Error()).ServeHTTP(w, r)
		return
	} else if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	token, err := ctrl.tokenSvc.Issue(user)
	if err != nil {
		problem.NewInternalServerError(err).ServeHTTP(w, r)
		return
	}

	// TODO: harden the cookie policy to the same domain
	cookie := &http.Cookie{
		Path:     "/",
		HttpOnly: true,
		Name:     "authorization",
		Value:    token,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctrl userController) userMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if t := r.Header.Get("Authorization"); t != "" {
			token = t
		} else if cs := r.CookiesNamed("authorization"); len(cs) > 0 {
			token = cs[0].Value // TODO: Validate cookie
		}

		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// TODO: Create some way to show the user what error occurred with the token,
		// not just the Unathorize method of UserContext. Maybe a web socket to send
		// the message? Or maybe a custom Header? A header can be intercepted via a
		// listener in the HTMX framework probably.

		ctx := r.Context()

		t, err := ctrl.tokenSvc.Parse(token)
		if err != nil {
			ctx = context.WithValue(ctx, "x-comicverse-user-token-error", err)
		} else {
			ctx = context.WithValue(ctx, "x-comicverse-user-token", t)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var _ middleware.Middleware = userController{}.userMiddleware

type UserContext struct {
	context.Context
}

func NewUserContext(ctx context.Context) UserContext {
	if uctxp, ok := ctx.(*UserContext); ok && uctxp != nil {
		return *uctxp
	} else if uctx, ok := ctx.(UserContext); ok {
		return uctx
	}
	return UserContext{Context: ctx}
}

func (ctx UserContext) Unathorize(w http.ResponseWriter, r *http.Request) {
	// TODO: Add a way to redirect to the login page in case of a incorrect token.
	// Since we use HTMX, we can't just return a redirect response probably,
	// the framework will just get the login page html and not redirect the user to the page.

	var p problem.Problem
	if err, ok := ctx.GetTokenErr(); ok {
		p = problem.NewUnauthorized(problem.AuthSchemeBearer, problem.WithError(err))
	} else {
		p = problem.NewUnauthorized(problem.AuthSchemeBearer)
	}

	p.ServeHTTP(w, r)
}

func (ctx UserContext) GetUserID() (uuid.UUID, bool) {
	claims, ok := ctx.GetClaims()
	if !ok {
		return uuid.UUID{}, false
	}

	sub, ok := claims["sub"]
	if !ok {
		return uuid.UUID{}, false
	}

	s, ok := sub.(string)
	if !ok {
		return uuid.UUID{}, false
	}

	id, err := uuid.Parse(s)
	if err != nil {
		// TODO?: Add error to error context
		return uuid.UUID{}, false
	}

	return id, true
}

func (ctx UserContext) GetClaims() (jwt.MapClaims, bool) {
	token, ok := ctx.GetToken()
	if !ok {
		return jwt.MapClaims{}, false
	}

	// TODO: Make claims type be registered in the user service
	// TODO: Structure claims type
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return jwt.MapClaims{}, false
	}

	return claims, true
}

func (ctx UserContext) GetToken() (*jwt.Token, bool) {
	t := ctx.Value("x-comicverse-user-token")
	if t == nil {
		return nil, false
	}

	token, ok := t.(*jwt.Token)
	if !ok {
		return nil, false
	}

	return token, true
}

func (ctx UserContext) GetTokenErr() (error, bool) {
	e := ctx.Value("x-comicverse-user-token-error")
	if e == nil {
		return nil, false
	}

	err, ok := e.(error)
	if !ok {
		return nil, false
	}

	return err, true
}
