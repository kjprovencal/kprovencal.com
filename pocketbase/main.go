package main

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

var app = pocketbase.New()

func main() {

	// load environment variables
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal("Error getting current directory: " + err.Error())
	}

	if err := godotenv.Load(dir + "/pocketbase/.env"); err != nil {
		log.Fatal("Error loading .env file: " + err.Error())
	}
	// loosely check if it was executed using "go run"
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Admin UI
		// (the isGoRun check is to enable it only during development)
		Dir:         "./pb_migrations",
		Automigrate: isGoRun,
	})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))

		e.Router.POST("/contact", contact)
		e.Router.GET("/contact", func(c echo.Context) error {
			err := sendEmail("Kyle", os.Getenv("EMAIL"), "Hello", "Hello, World!")
			if err != nil {
				return apis.NewApiError(500, "Error sending email", err)
			} else {
				return c.JSON(200, map[string]interface{}{
					"message": "Email sent",
				})
			}
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

}
