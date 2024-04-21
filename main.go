package main

import (
	"DistributedCalculator/agent"
	"DistributedCalculator/db"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Log the handler call
	log.Println("Index handler")

	// Parse the HTML templates
	tmpl, err := template.ParseFiles("templates/base.html", "templates/index.html")
	// Check if there was an error parsing the templates
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	// Create a data structure to hold the page title
	data := struct {
		Title string
	}{
		Title: "Добавить выражение",
	}

	// Execute the template with the data
	err = tmpl.ExecuteTemplate(w, "base.html", data)
	// Check if there was an error executing the template
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func addEquationHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
	if r.Method == "POST" {
		// Log the handler call
		log.Println("addEquationHandler")

		// Parse the form data from the request
		err := r.ParseForm()
		// Check if there was an error parsing the form
		if err != nil {
			// Send an HTTP 500 error and log the error
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Fatal(err)
			return
		}

		// Get the id and text from the form data
		idStr := r.FormValue("id")
		text := r.FormValue("text")
		// Connect to the database
		database, err := db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
			return
		}
		// Ensure the database connection is closed when the function returns
		defer func() {
			if err = database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		id := 0
		if idStr != "" {
			// Convert the id to an integer
			id, err = strconv.Atoi(idStr)
			// Redirect to the get route with the id
			http.Redirect(w, r, fmt.Sprintf("/get/%d", id), http.StatusSeeOther)
		} else {
			// Check if the equation is valid
			if !agent.ValidEquation(text, 0, len(text)) {
				// Send an HTTP 400 error and log the error
				http.Error(w, "Invalid equation", http.StatusBadRequest)
				log.Println("Invalid equation")
				return
			}

			// Add the equation to the database
			id, err = database.AddEquation(id, text, "Equations")
			if err != nil {
				log.Fatal(err)
			}

			// Evaluate the equation in a goroutine
			go func() {
				err = agent.Evaluate(id)
				if err != nil {
					log.Fatal(err)
				}
			}()

			// Redirect to the root route
			http.Redirect(w, r, "/equations", http.StatusSeeOther)
		}
	}
}

// getEquationHandler handles the "/get/" route and retrieves an equation from the database based on its ID.
// It first parses the URL path to get the ID, then connects to the database.
// If the equation with the given ID is found, it is returned as a JSON response.
// If the equation is not found, it sends an HTTP 404 error.
func getEquationHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the URL path to get the id
	path := strings.Split(r.URL.Path, "/")
	idStr := path[len(path)-1]

	// Convert the id to an integer
	id, err := strconv.Atoi(idStr)
	if err != nil {
		// Send an HTTP 400 error for invalid ID
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Connect to the database
	var database *db.DB
	database, err = db.Connect("data.db")
	if err != nil {
		log.Fatal(err)
		return
	}
	// Ensure the database connection is closed when the function returns
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Get the equation with the given id
	equation, status, result := database.GetEquationInfo(id)
	if err != nil {
		// Send an HTTP 404 error for equation not found
		http.Error(w, "Equation not found", http.StatusNotFound)
		return
	}
	// Prepare the JSON response
	var jsonStr []byte
	jsonStr, err = json.Marshal(map[string]interface{}{
		"id":     id,
		"text":   equation,
		"status": status,
		"result": result,
	})
	if err != nil {
		// Send an HTTP 500 error for internal server error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
	// Set the content type of the response to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	_, err = w.Write(jsonStr)
	if err != nil {
		log.Fatal(err)
	}
}
func equationsHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	database, err := db.Connect("data.db")
	if err != nil {
		// Log the error and terminate the program
		log.Fatal(err)
	}
	// Ensure the database connection is closed when the function returns
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Retrieve all equations from the database
	var values []map[string]interface{}
	values, err = database.GetAllValues("Equations")
	if err != nil {
		log.Fatal(err)
	}

	// Parse the HTML templates
	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates/base.html", "templates/equations.html")
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	// Create a data structure to hold the page title and equations
	data := struct {
		Title     string
		Equations []map[string]interface{}
	}{
		Title:     "Выражения",
		Equations: values,
	}

	// Execute the template with the data
	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func operationsHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	database, err := db.Connect("data.db")
	if err != nil {
		// Log the error and terminate the program
		log.Fatal(err)
	}
	// Ensure the database connection is closed when the function returns
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Retrieve all operations from the database
	var values []map[string]interface{}
	values, err = database.GetAllValues("Operations")
	if err != nil {
		log.Fatal(err)
	}

	// Parse the HTML templates
	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates/base.html", "templates/operations.html")
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	// Create a data structure to hold the page title and operations
	data := struct {
		Title      string
		Operations []map[string]interface{}
	}{
		Title:      "Операции",
		Operations: values,
	}

	// Execute the template with the data
	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}
func updateOperationsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
	if r.Method == "POST" {
		// Parse the form data from the request
		err := r.ParseForm()
		// Check if there was an error parsing the form
		if err != nil {
			// Send an HTTP 500 error and log the error
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Get the operation times from the form data
		plusTime := r.FormValue("time_+")
		minusTime := r.FormValue("time_-")
		multiplyTime := r.FormValue("time_*")
		divideTime := r.FormValue("time_/")

		// Connect to the database
		database, err := db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
		}
		// Ensure the database connection is closed when the function returns
		defer func() {
			if err := database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		// Define the operation types and times
		types := []string{"+", "-", "*", "/"}
		times := []string{plusTime, minusTime, multiplyTime, divideTime}

		// Update the operation times in the database
		err = database.UpdateOperations(types, times)
		if err != nil {
			log.Fatal(err)
		}

		// Redirect to the operations page
		http.Redirect(w, r, "/operations", http.StatusSeeOther)
	}
}
func computersHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	database, err := db.Connect("data.db")
	if err != nil {
		// Log the error and terminate the program
		log.Fatal(err)
	}
	// Ensure the database connection is closed when the function returns
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Retrieve all computers from the database
	var values []map[string]interface{}
	values, err = database.GetAllValues("Computers")
	if err != nil {
		log.Fatal(err)
	}

	// Parse the HTML templates
	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates/base.html", "templates/computers.html")
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	// Create a data structure to hold the page title and computers
	data := struct {
		Title     string
		Computers []map[string]interface{}
	}{
		Title:     "Вычислители",
		Computers: values,
	}

	// Execute the template with the data
	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		// Send an HTTP 500 error and log the error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

// addComputerHandler handles the "/add_computer" route and adds a new computer to the database.
// It first checks if the request method is POST.
// If it is, it parses the form data from the request.
// Then it connects to the database and adds a new computer.
// Finally, it redirects to the "/computers" route.
func addComputerHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
	if r.Method == "POST" {
		// Parse the form data from the request
		err := r.ParseForm()
		// Check if there was an error parsing the form
		if err != nil {
			// Send an HTTP 500 error and log the error
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Fatal(err)
			return
		}

		// Connect to the database
		var database *db.DB
		database, err = db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
		}
		// Ensure the database connection is closed when the function returns
		defer func() {
			if err = database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		// Add a new computer to the database
		err = database.AddComputer()
		if err != nil {
			log.Fatal(err)
		}

		// Redirect to the computers page
		http.Redirect(w, r, "/computers", http.StatusSeeOther)
	}
}

func main() {
	// Check if database exists and create it if it doesn't
	_, err := os.Stat("data.db")
	if os.IsNotExist(err) {
		// Create the database file
		var file *os.File
		file, err = os.Create("data.db")
		if err != nil {
			fmt.Println(err)
		}
		// Close the file after creating it
		err = file.Close()
		if err != nil {
			fmt.Println(err)
		}
	}
	// Connect to the database
	database, _ := db.Connect("data.db")
	// Initialize the database
	err = database.Init()
	if err != nil {
		fmt.Println(err)
	}
	// Close the database connection
	err = database.Close()
	if err != nil {
		fmt.Println(err)
	}

	// Open the log file
	logFile, err := os.OpenFile(".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
	}
	// Set the log output to the log file
	log.SetOutput(logFile)
	// Log that the server has started
	log.Println("Server started")

	// Define the HTTP routes and their handlers
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/add_equation", addEquationHandler)
	http.HandleFunc("/get/", getEquationHandler)
	http.HandleFunc("/equations", equationsHandler)
	http.HandleFunc("/operations", operationsHandler)
	http.HandleFunc("/computers", computersHandler)
	http.HandleFunc("/update_operations", updateOperationsHandler)
	http.HandleFunc("/add_computer", addComputerHandler)

	// Start the HTTP server
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
