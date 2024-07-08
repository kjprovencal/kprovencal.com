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

func sendEmail(name, subject, body string) error {
	// send email using SendGrid
	from := mail.NewEmail(name, os.Getenv("SYS_EMAIL"))
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
	_, err := app.Dao().DB().
		Insert("contact", map[string]interface{}{"name": data[0], "email": data[1]}).
		Execute()

	if err != nil {
		app.Logger().Error("Error recording contact: " + err.Error())
	}
}

func contact(c echo.Context) error {
	admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
	record, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)
	if isGuest := admin == nil && record == nil; isGuest {
		data := apis.RequestInfo(c).Data
		name := data["name"].(string)
		email := data["email"].(string)

		if checkSubmitted(name, email) {
			return apis.NewApiError(400, "Contact already submitted", nil)
		} else {
			recordContact(name, email)
		}

		subject := data["subject"].(string)
		message := "New message from " + name + " <" + email + ">\n\n"

		if data["message"] != nil {
			message += data["message"].(string)
		}

		message += "\n\n--\nSent from PocketBase\n"

		// send me an email
		if err := sendEmail(name, subject, message); err != nil {
			app.Logger().Error("Error sending email: " + err.Error())
			return apis.NewApiError(500, "Error sending email", err)
		}

		app.Logger().Info("Email sent. New contact: " + name + " <" + email + ">")
		return c.JSON(200, map[string]interface{}{
			"message": "Email sent",
		})
	}
	if admin != nil {
		app.Logger().Info("Admin is not allowed to contact.")
	} else if record != nil {
		app.Logger().Info("User is not allowed to contact.")
	}
	return apis.NewUnauthorizedError("Unauthorized", nil)
}

