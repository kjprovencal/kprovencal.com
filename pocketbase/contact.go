package main

import (
	"html"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/models"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func sendEmail(name, email, subject, body string) error {
	// send email using SendGrid
	from := mail.NewEmail(name, email)
	to := mail.NewEmail(os.Getenv("NAME"), os.Getenv("EMAIL"))
	plainTextContent := body
	htmlContent := "<strong>" + html.EscapeString(body) + "</strong>"
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	res, err := client.Send(message)
	println(res.Body)
	return err
}

func checkSubmitted(data ...string) bool {
	// check if contact has already submitted
	record, err := app.Dao().FindFirstRecordByData("contact", "email", data[1])

	return record != nil && err == nil
}

func recordContact(data ...string) {
	// record contact in database
	res, err := app.Dao().DB().
		Insert("contact", map[string]interface{}{"name": data[0], "email": data[1]}).
		Execute()

	if err != nil {
		println(res)
	}
}

func contact(c echo.Context) error {
	admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

	if isGuest := admin == nil && record == nil; isGuest {
		if token := c.Request().Header.Get("Authorization"); token != "" {
			data := apis.RequestInfo(c).Data
			name := data["name"].(string)
			email := os.Getenv("EMAIL")

			if checkSubmitted(name, email) {
				return apis.NewApiError(400, "Contact already submitted", nil)
			} else {
				recordContact(name, email)
			}

			subject := data["subject"].(string)
			message := "New message from " + name + " <" + email + ">\n\n" + data["message"].(string) + "\n\n--\nSent from PocketBase\n"
			// send me an email
			if err := sendEmail(name, email, subject, message); err != nil {
				return apis.NewApiError(500, "Error sending email", err)
			}
			return c.JSON(200, map[string]interface{}{
				"message": "Email sent",
			})
		} else {
			// not authorized, return 401
			return apis.NewUnauthorizedError("Unauthorized", nil)
		}
	}
	return nil
}
