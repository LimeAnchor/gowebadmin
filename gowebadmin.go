package gowebadmin

import (
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v72"
	"html/template"
)

// Router für Login

// Router für das bezahlen

/*
	router.Any("/create-portal-session", payment.Wrap(payment.CreatePortalSession))
	router.Any("/create-customer-portal-session", payment.CheckoutWrap(payment.CreatePortalSession))
	router.Any("/webhook", payment.Wrap(payment.HandleWebhook2))
	router.GET("/checkout", payment.Checkout)
	router.GET("/success", payment.Success)
	router.GET("/success.html", payment.Success)
	router.GET("/cancel", payment.Cancel)
*/

/*
	router.GET("/login", login.LoginHandler(auth))
	router.GET("/callback", login.CallbackHandler(auth))
	router.GET("/logout", login.LogoutHandler)
*/

type Customer struct {
	StripeAccount      string
	AuthO              string
	EMail              string
	SubscribedProducts []*stripe.Subscription
	PaymentValid       bool
	MailVerified       bool
}

type WebAdmin struct {
	Domain   string
	Database Database
	Stripe   StripeConfig
	Auth0    Auth0
}

type Auth0 struct {
	Domain        string
	ClientId      string
	ClientSecret  string
	Callback      string
	AfterLogin    string
	Authenticator *Authenticator
}

type StripeConfig struct {
	CheckoutTitle    string
	CheckoutTemplate *template.Template
	PricingTabelId   string
	PublishabelKey   string
	StripeKey        string
	EndpointSecret   string
	WebhookSecret    string
	LookupKey        string
	CustomEndpoints  CustomEndpoints
	Pages            Pages
}

type Pages struct {
	Checkout Page
	Success  Page
	Cancel   Page
}

type Page struct {
	Path string
	File string
}

type Database struct {
	ConnectionString string
	Database         string
}

type CustomEndpoints struct {
	SuccessUrl string
	CancelUrl  string
	ReturnUrl  string
}

func (web *WebAdmin) GetRouters(router *gin.Engine) {
	router.Any("/create-portal-session", web.CustomerWrap(web.CreatePortalSession))
	router.Any("/create-customer-portal-session", web.CustomerWrap(web.CreatePortalSession))
	router.Any("/webhook", Wrap(web.HandleWebhook))
	router.GET("/login", web.LoginHandler(web.Auth0.Authenticator))
	router.GET("/callback", web.CallbackHandler(web.Auth0.Authenticator))
	router.GET("/logout", web.LogoutHandler)
	router.GET("/checkout", web.Checkout)
	router.GET("/verify", web.VerifyEmailBlock)
}

func Gowebadmin(domain string, db Database, stripe StripeConfig, auth Auth0) *WebAdmin {
	// Init Webpages
	SetStripeKey(stripe.StripeKey)
	if stripe.CustomEndpoints.SuccessUrl == "" {
		stripe.CustomEndpoints.SuccessUrl = "/success"
	}
	if stripe.CustomEndpoints.CancelUrl == "" {
		stripe.CustomEndpoints.CancelUrl = "/cancel"
	}
	if stripe.CustomEndpoints.ReturnUrl == "" {
		stripe.CustomEndpoints.ReturnUrl = "/config"
	}
	if stripe.Pages.Checkout.File == "" {
		stripe.Pages.Checkout.File = "checkout.html"
	}
	if stripe.Pages.Checkout.Path == "" {
		stripe.Pages.Checkout.Path = "/checkout"
	}
	if stripe.Pages.Success.File == "" {
		stripe.Pages.Success.File = "success.html"
	}
	if stripe.Pages.Success.Path == "" {
		stripe.Pages.Success.Path = "/success"
	}
	if stripe.Pages.Cancel.File == "" {
		stripe.Pages.Cancel.File = "cancel.html"
	}
	if stripe.Pages.Cancel.Path == "" {
		stripe.Pages.Cancel.Path = "/cancel"
	}
	web := &WebAdmin{
		domain,
		db,
		stripe,
		auth,
	}
	web.InitStripeCheckout()
	return web
}

func (web *WebAdmin) AddCustomer() {

}

func (web *WebAdmin) Validate() {

}
