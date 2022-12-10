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
	"github.com/stripe/stripe-go/v72/price"
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
		profil := web.GetOne("users", bson.M{"EMail": valStr}).Customer()
		c.Request.Header.Add("customer", profil.StripeAccount)
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
	lookup_key := r.PostFormValue(web.Stripe.LookupKey)
	domain := os.Getenv(web.Domain)
	params := &stripe.PriceListParams{
		LookupKeys: stripe.StringSlice([]string{
			lookup_key,
		}),
	}
	i := price.List(params)
	var price *stripe.Price
	for i.Next() {
		p := i.Price()
		price = p
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				Price:    stripe.String(price.ID),
				Quantity: stripe.Int64(1),
			},
		},

		SuccessURL: stripe.String(domain + web.Stripe.CustomEndpoints.SuccessUrl + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(domain + web.Stripe.CustomEndpoints.CancelUrl),
	}
	if customerId != "" {
		checkoutParams.Customer = &customerId
	}

	s, err := session.New(checkoutParams)
	if err != nil {
		log.Printf("session.New: %v", err)
	}

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func (web *WebAdmin) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	returnurl := "https://" + web.Domain + "/admin"
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
	profil := web.GetOne("users", bson.M{"EMail": c.Email}).Customer()
	profil.StripeAccount = custId
	enterprisedomains := make(map[string]int)
	advanceddomains := make(map[string]int)
	starterdomains := make(map[string]int)

	for _, sub := range c.Subscriptions.Data {
		if sub.Status == "active" {
			for _, item := range sub.Items.Data {
				for i := 0; i < int(item.Quantity); i++ {
					if item.Plan.Nickname == "staa" || item.Plan.Nickname == "stam" {
						starterdomains[sub.ID]++
					} else if item.Plan.Nickname == "adva" || item.Plan.Nickname == "advm" {
						advanceddomains[sub.ID]++
					} else if item.Plan.Nickname == "entm" || item.Plan.Nickname == "enta" {
						enterprisedomains[sub.ID]++
					}
				}
			}
		}
	}
	profil.SubscribedProducts = c.Subscriptions.Data
	// jetzt muss ich die domains anlegen
	var newdomains []Domains

	for _, domain := range profil.Domains {
		if domain.MaxSites == 10 {
			if starterdomains[domain.LinkedSubscription] == 0 {
				// Deaktiviere, wenn nicht mehr vorhanden
				domain.Active = false
				if domain.Address != "" {
					newdomains = append(newdomains, domain)
				}
			} else {
				starterdomains[domain.LinkedSubscription]--
			}
		} else if domain.MaxSites == 25 {
			if advanceddomains[domain.LinkedSubscription] == 0 {
				// Deaktiviere, wenn nicht mehr vorhanden
				domain.Active = false
				if domain.Address != "" {
					newdomains = append(newdomains, domain)
				}
			} else {
				advanceddomains[domain.LinkedSubscription]--
			}
		} else if domain.MaxSites == 60 {
			if enterprisedomains[domain.LinkedSubscription] == 0 {
				// Deaktiviere, wenn nicht mehr vorhanden

				domain.Active = false
				if domain.Address != "" {
					newdomains = append(newdomains, domain)
				}

			} else {
				enterprisedomains[domain.LinkedSubscription]--
			}
		}
	}

	for enter, _ := range enterprisedomains {
		newdomains = append(newdomains, Domains{
			MaxSites:           60,
			LinkedSubscription: enter,
			Active:             true,
		})

	}
	for enter, _ := range advanceddomains {
		newdomains = append(newdomains, Domains{
			MaxSites:           25,
			LinkedSubscription: enter,
			Active:             true,
		})
	}
	for enter, _ := range starterdomains {
		newdomains = append(newdomains, Domains{
			MaxSites:           10,
			LinkedSubscription: enter,
			Active:             true,
		})
	}

	profil.Domains = newdomains

	web.Upsert("users", profil, bson.D{{"EMail", c.Email}}, true)
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
			<style>
				.PricingTable.is-blackButtonText .PriceColumn-button {
					color: white !important;
				}
			</style>
		</head>
		<body style="background-color: white !important;padding: 150px;">
			<script async src="https://js.stripe.com/v3/pricing-table.js"></script>
			<stripe-pricing-table 
				pricing-table-id="{{ .PricingTableId }}"
				publishable-key="{{ .PublishableKey }}" 
				customer-email="{{.CustomerEmail}}" 
				{{ if ne .Customer ""}} 
					customer="{{ .Customer }}"
				{{ end }}>
			</stripe-pricing-table>
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
	}{
		PricingTableId: web.Stripe.PricingTabelId,
		PublishableKey: web.Stripe.PublishabelKey,
		CustomerEmail:  mail,
		Title:          web.Stripe.CheckoutTitle,
	}
	if customer != "" {
		x.Customer = customer
	}

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

	//Get name from profile and search for entry in database
	valStr := GetName(profile)
	profil := web.GetOne("users", bson.M{"EMail": valStr}).Customer()
	if profil.StripeAccount == "" {
		ctx.Redirect(http.StatusSeeOther, "/checkout")
		//ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(web.RenderTemplate(valStr)))
		return
	}
}

func (web *WebAdmin) Checkout(ctx *gin.Context) {
	session := sessions.Default(ctx)
	profile := session.Get("profile")
	valStr := GetName(profile)
	profil := web.GetOne("users", bson.M{"EMail": valStr}).Customer()
	//web.CreateCheckoutSessionBasic(ctx.Writer, ctx.Request)
	fmt.Println("Stripe", profil.StripeAccount)
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(web.RenderTemplate(valStr, profil.StripeAccount)))
}
