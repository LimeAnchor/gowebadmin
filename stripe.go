package gowebadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v72"
	portalsession "github.com/stripe/stripe-go/v72/billingportal/session"
	"github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/customer"
	"github.com/stripe/stripe-go/webhook"
	"go.mongodb.org/mongo-driver/bson"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func SetStripeKey(key string) {
	stripe.Key = key
}



/*
func (cust *Customer) CheckCustomer(customerid string) error {
	cust.StripeAccount = customerid
	c, err := customer.Get(customerid, nil)
	if err != nil {
		return err
	}
	if c == nil {
		return errors.New("Customer not found")
	}
	// Add all subscriptions
	for _, subs := range c.Subscriptions.Data {
		for _, data := range subs.Items.Data {
			if data.Plan != nil {
				cust.SubscribedProducts = append(cust.SubscribedProducts, data.Plan.Nickname)
			}
		}
	}
	return nil
}

func (customer Customer) CheckCustomerSubscription() []string {
	return customer.SubscribedProducts
}*/

func Wrap(f http.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		f(c.Writer, c.Request)
	}
}

func (web *WebAdmin) CustomerWrap(f http.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		profile := session.Get("profile")
		valStr := GetName(profile)
		profil := web.GetOne(web.Collection, bson.M{web.MailTitle: valStr}).Customer()
		c.Request.Header.Add("customer", profil.AboDetails)
		f(c.Writer, c.Request)
	}
}

func (web *WebAdmin) CreateCheckoutSessionBasic(w http.ResponseWriter, r *http.Request) {
	customerId := r.Header.Get("customer")
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	domain := os.Getenv(web.Domain)

	checkoutParams := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Customer:   &customerId,
		SuccessURL: stripe.String(domain + web.Stripe.CustomEndpoints.SuccessUrl + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(domain + web.Stripe.CustomEndpoints.CancelUrl),
	}
	s, err := session.New(checkoutParams)
	if err != nil {
		log.Printf("session.New: %v", err)
	}
	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (web *WebAdmin) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	returnurl := web.Domain + "/"
	customerId := r.Header.Get("customer")

	// Authenticate your user.
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerId),
		ReturnURL: stripe.String(returnurl),
	}
	ps, err := portalsession.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, ps.URL, http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("json.NewEncoder.Encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Printf("io.Copy: %v", err)
		return
	}
}

func (web *WebAdmin) HandleWebhook(w http.ResponseWriter, req *http.Request) {
	const MaxBodyBytes = int64(65536)
	bodyReader := http.MaxBytesReader(w, req.Body, MaxBodyBytes)
	payload, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	endpointSecret := web.Stripe.EndpointSecret
	signatureHeader := req.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event.Type {
	case "customer.subscription.deleted":
		var subscription stripe.Subscription
		err = json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		web.UpdateCustomer(subscription)
	case "customer.subscription.updated":
		var subscription stripe.Subscription
		err = json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		web.UpdateCustomer(subscription)
	case "customer.subscription.created":
		var subscription stripe.Subscription
		err = json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		web.UpdateCustomer(subscription)
	case "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err = json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		web.UpdateCustomer(subscription)
	default:
	}
	w.WriteHeader(http.StatusOK)
}

func (web *WebAdmin) UpdateCustomer(sub stripe.Subscription) {
	custId := sub.Customer.ID
	// Check Customer
	params := stripe.CustomerParams{}
	params.AddExpand("subscriptions")

	c, err := customer.Get(custId, &params)
	if err != nil {
		fmt.Println("Check Costumer failed " + err.Error())
	}
	if c == nil {
		fmt.Println("Customer not found ")
	}
	profil := web.GetOne(web.Collection, bson.M{web.MailTitle: c.Email}).Customer()
	profil.StripeAccount = custId
	profil.SubscribedProducts = c.Subscriptions.Data



	// jetzt muss ich die domains anlegen
	web.Upsert(web.Collection, profil, bson.D{{web.MailTitle, c.Email}}, true)
}

var tmpl *template.Template

func (web *WebAdmin) InitStripeCheckout() {
	s := `
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
    <link rel="stylesheet" href="/public/style.css">
    <script src="https://polyfill.io/v3/polyfill.min.js?version=3.52.1&features=fetch"></script>
    <script src="https://js.stripe.com/v3/"></script>
</head>
<style>
    .PricingTable.is-blackButtonText .PriceColumn-button {
        color: white !important;
    }
    {{ .Customcss }}
</style>
<body style="padding: 150px;">
{{ .Customhead }}
<script async src="https://js.stripe.com/v3/pricing-table.js"></script>
<stripe-pricing-table pricing-table-id="{{ .PricingTableId }}"
                      publishable-key="{{ .PublishableKey }}" customer-email="{{.CustomerEmail}}">
</stripe-pricing-table>
{{ .Customfooter }}
</body>
</html>
	`
	web.Stripe.CheckoutTemplate = template.Must(template.New("request.tmpl").Parse(s))
}

func (web *WebAdmin) RenderTemplate(mail string, customer string) string {
	x := struct {
		PricingTableId string
		PublishableKey string
		CustomerEmail  string
		Title          string
		Customer       string
		Customcss string
		Customhead string
		Customfooter string
	}{
		PricingTableId: web.Stripe.PricingTabelId,
		PublishableKey: web.Stripe.PublishabelKey,
		CustomerEmail:  mail,
		Title:          web.Stripe.CheckoutTitle,
		Customcss: web.Stripe.CustomCss,
		Customhead: web.Stripe.CustomHead,
		Customfooter: web.Stripe.CustomBody,
	}
	if customer != "" {
		x.Customer = customer
	}
	fmt.Println("Render Tempalte Mail ", mail)

	var tpl bytes.Buffer
	err := web.Stripe.CheckoutTemplate.ExecuteTemplate(&tpl, "request.tmpl", x)
	if err != nil {
		fmt.Println(err)
	}
	return strings.ReplaceAll(strings.ReplaceAll(tpl.String(), "\n", ""), "  ", " ")
}
func (web *WebAdmin) IsCustomer(ctx *gin.Context) {

	// Check of user exists and create if not
	// If user not found in database, create it
	session := sessions.Default(ctx)
	profile := session.Get("profile")
	fmt.Println("try check out with profile: ", profile)
	//Get name from profile and search for entry in database
	valStr := GetName(profile)
	fmt.Println("try check out with: ", valStr)
	profil := web.GetOne(web.Collection, bson.M{web.MailTitle: valStr}).Customer()
	if profil.StripeAccount == "" && profil.AboDetails == "" {
		fmt.Println("Forward to checkout: ", valStr)
		ctx.Redirect(http.StatusSeeOther, "/checkout")
		return
	}
}

func (web *WebAdmin) Checkout(ctx *gin.Context) {
	session := sessions.Default(ctx)
	profile := session.Get("profile")
	fmt.Println("CH: try check out with profile: ", profile)
	valStr := GetName(profile)
	fmt.Println("CH: try check out with: ", valStr)
	profil := web.GetOne(web.Collection, bson.M{web.MailTitle: valStr}).Customer()
	account := profil.StripeAccount
	if profil.StripeAccount == "" {
		account = profil.AboDetails
	}
	fmt.Println("used mail is ",account )
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(web.RenderTemplate(valStr,account)))
}