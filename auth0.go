package gowebadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

func GetName(in interface{}) string {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Map {
		for _, key := range v.MapKeys() {
			strct := v.MapIndex(key)
			if key.Interface() == "name" {
				return fmt.Sprintf("%v", strct.Interface())
			}
		}
	}
	return ""
}

func GetID(in interface{}, typ string) string {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Map {
		for _, key := range v.MapKeys() {
			strct := v.MapIndex(key)
			if key.Interface() == typ {
				return fmt.Sprintf("%v", strct.Interface())
			}
		}
	}
	return ""
}

func GetBool(in interface{}, typ string) bool {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Map {
		for _, key := range v.MapKeys() {
			strct := v.MapIndex(key)
			if key.Interface() == typ {
				return strct.Interface().(bool)
			}
		}
	}
	return false
}

func CheckUserExists(profile Customer) bool {
	if profile.EMail == "" && profile.Title == "" {
		return false
	}
	return true
}

func (web *WebAdmin) IsAuthenticated(ctx *gin.Context) {
	// Check of user exists and create if not
	// If user not found in database, create it
	session := sessions.Default(ctx)
	profile := session.Get("profile")
	if profile == nil {
		ctx.Redirect(http.StatusSeeOther, "/login")
		return
	}
	//Get name from profile and search for entry in database
	valStr := GetName(profile)

	profil := web.GetOne(web.Collection, bson.M{"EMail": valStr}).Customer()
	if !CheckUserExists(profil) {
		profil.EMail = valStr
		profil.Title = valStr
		web.InsertOne(web.Collection, profil)
	}

	if !profil.MailVerified {
		auth0user := web.CheckUser(web.GetToken(), valStr)
		if auth0user.EmailVerified {
			profil.MailVerified = true
			web.Upsert(web.Collection, profil, bson.D{{web.MailTitle, valStr}}, true)
		} else {
			ctx.Redirect(http.StatusSeeOther, web.VerifyPath)
		}

	}

	if sessions.Default(ctx).Get("profile") == nil {
		ctx.Redirect(http.StatusSeeOther, "/")
	} else {
		ctx.Next()
	}
}

func (web *WebAdmin) VerifyEmailBlock(ctx *gin.Context) {
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
		  <meta charset="UTF-8">
		  <link rel="stylesheet" href="/public/styles.css">
		  </head>
		<body style="background-color:#213c5e">
		<section style="padding: 300px;min-height: 100%; color: white">
		  <div class="product Box-root">
			<div class="description Box-root">
			  <h3>Bitte bestätige zunächst Deine E-Mail Adresse</h3>
			</div>
		  </div>
		  <br>
		  <form action="/admin" method="GET">
			<input type="hidden" id="session-id" name="session_id" value="" />
			<button class="btn btn-light" id="checkout-and-portal-button" type="submit">Erneut prüfen</button>
		  </form>
		</section>
		</body>
	</html>
	`))
}

type Subscription struct {
	Name string `json:"name"`
}

func (web *WebAdmin) GetSubscription(userid string) []string {
	url := "https://" + web.Auth0.Domain + "/api/v2/users/" + userid + "/roles"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("authorization", "Bearer "+web.GetToken())
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	var subscriptions []Subscription
	json.Unmarshal(body, &subscriptions)
	data := []string{}
	for _, sub := range subscriptions {
		data = append(data, sub.Name)
	}
	return data
}

func (web *WebAdmin) GetToken() string {
	url := "https://" + web.Auth0.Domain + "/oauth/token"
	s := "{\"client_id\":\"" + web.Auth0.ClientIdAPI + "\",\"client_secret\":\"" + web.Auth0.ClientSecretAPI + "\",\"audience\":\"https://" + web.Auth0.Domain + "/api/v2/\",\"grant_type\":\"client_credentials\"}"
	payload := strings.NewReader(s)
	req, err := http.NewRequest("POST", url, payload)
	fmt.Println(s)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(s)
		return ""
	}
	req.Header.Add("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(s)
		return ""
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(string(body))
		return ""
	}
	var token Token

	err = json.Unmarshal(body, &token)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(string(body))
		return ""
	}
	fmt.Println(string(body))
	return token.AccessToken
}

type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (web *WebAdmin) CheckUser(token string, stripeEmail string) *Auth0user {
	// Step 1: User per Mail finden
	req, err := http.NewRequest("GET", "https://"+web.Auth0.Domain+"/api/v2/users-by-email?include_fields=true&email="+stripeEmail, nil)
	if err != nil {
		fmt.Println("Request Create Error " + err.Error())
	}
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Request Send Error " + err.Error())
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Read Body " + err.Error())
	}
	users := []Auth0user{}
	json.Unmarshal(body, &users)
	if len(users) == 1 {
		return &users[0]
	} else {
		fmt.Println(string(body))
	}
	return nil
}

type Auth0user struct {
	UserID        string `json:"user_id"`
	EmailVerified bool   `json:"email_verified"`
}

func (web *WebAdmin) UpdateSubscription(stripeEmail string, remove bool, role string) {
	token := web.GetToken()
	user := web.CheckUser(token, stripeEmail)
	if remove {
		web.DeleteRole(token, role, user.UserID)
	} else {
		web.UpdateRole(token, role, user.UserID)
	}
}

func (web *WebAdmin) AddStripeCustomerID(token string, userid string, customerId string) {
	user := struct {
		Nickname string `json:"nickname"`
	}{
		Nickname: customerId,
	}
	url := "https://" + web.Auth0.Domain + "/api/v2/users/" + userid

	b, err := json.Marshal(user)
	if err != nil {
		fmt.Println("Error marshaling struct " + err.Error())
	}
	req, err := http.NewRequest("PATCH", url, strings.NewReader(string(b)))
	if err != nil {
		fmt.Println("Error creating request " + err.Error())
	}
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending request " + err.Error())
	}
	defer res.Body.Close()
}

func (web *WebAdmin) UpdateRole(token string, role string, userid string) {
	user := UserObject{
		Users: []string{userid},
	}
	url := "https://" + web.Auth0.Domain + "/api/v2/roles/" + role + "/users"
	b, err := json.Marshal(user)
	if err != nil {
		fmt.Println("Error marshaling struct " + err.Error())
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(b)))
	if err != nil {
		fmt.Println("Error creating request " + err.Error())
	}
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending request " + err.Error())
	}
	defer res.Body.Close()
}

func (web *WebAdmin) DeleteRole(token string, role string, userid string) {
	r := RoleObject{
		Roles: []string{role},
	}
	url := "https://" + web.Auth0.Domain + "/api/v2/users/" + userid + "/roles"
	b, err := json.Marshal(r)
	if err != nil {
		fmt.Println("Error marshaling struct " + err.Error())
	}
	req, err := http.NewRequest("DELETE", url, strings.NewReader(string(b)))
	if err != nil {
		fmt.Println("Error creating request " + err.Error())
	}
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error sending request " + err.Error())
	}
	defer res.Body.Close()
}

type UserObject struct {
	Users []string `json:"users"`
}

type RoleObject struct {
	Roles []string `json:"roles"`
}

// LogoutHandler will logout the user from auth0
func (web *WebAdmin) LogoutHandler(ctx *gin.Context) {
	logoutUrl, err := url.Parse("https://" + web.Auth0.Domain + "/v2/logout")
	if err != nil {
		fmt.Println(err.Error())
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	returnTo, err := url.Parse(web.Auth0.Domain)
	if err != nil {
		fmt.Println(err.Error())
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	session := sessions.Default(ctx)
	session.Clear()
	session.Options(sessions.Options{MaxAge: -1})

	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", web.Auth0.ClientId)
	logoutUrl.RawQuery = parameters.Encode()
	ctx.Redirect(http.StatusTemporaryRedirect, logoutUrl.String())
}

func (web *WebAdmin) LoginHandler(auth *Authenticator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		state, err := generateRandomState()
		if err != nil {
			fmt.Println(err.Error())
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		// Save the state inside the session.
		session := sessions.Default(ctx)
		session.Set("state", state)
		if err = session.Save(); err != nil {
			fmt.Println(err.Error())
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Redirect(http.StatusTemporaryRedirect, auth.AuthCodeURL(state))
	}
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}

// CallbackHandler manages the callback after Auth0 login
func (web *WebAdmin) CallbackHandler(auth *Authenticator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		if ctx.Query("state") != session.Get("state") {
			ctx.String(http.StatusBadRequest, "Invalid state parameter.")
			return
		}
		// Exchange an authorization code for a token.
		token, err := auth.Exchange(ctx.Request.Context(), ctx.Query("code"))
		if err != nil {
			ctx.String(http.StatusUnauthorized, "Failed to exchange an authorization code for a token.")
			return
		}

		idToken, err := auth.VerifyIDToken(ctx.Request.Context(), token)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to verify ID Token.")
			return
		}

		var profile map[string]interface{}
		if err = idToken.Claims(&profile); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		session.Set("access_token", token.AccessToken)
		session.Set("profile", profile)
		if err = session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Redirect(http.StatusTemporaryRedirect, web.Auth0.AfterLogin)
	}
}

// Authenticator is used to authenticate our users.
type Authenticator struct {
	*oidc.Provider
	oauth2.Config
}

// New instantiates the *Authenticator.
func (web *WebAdmin) Auth() {
	provider, _ := oidc.NewProvider(
		context.Background(),
		"https://"+web.Auth0.Domain+"/",
	)

	conf := oauth2.Config{
		ClientID:     web.Auth0.ClientId,
		ClientSecret: web.Auth0.ClientSecret,
		RedirectURL:  web.Auth0.Callback,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
	}
	web.Auth0.Authenticator = &Authenticator{
		Provider: provider,
		Config:   conf,
	}
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in oauth2 token")
	}

	oidcConfig := &oidc.Config{
		ClientID: a.ClientID,
	}

	return a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
}
