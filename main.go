package main

import (
	"DistributedCalculator/agent"
	"DistributedCalculator/db"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var hmacSampleSecret = []byte("super_secret_signature")

func RegisterAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var user struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 8)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	database, err := db.Connect("data.db")
	if err != nil {
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}
	defer database.Close()

	err = database.AddUser(user.Login, string(hashedPassword))
	if err != nil {
		http.Error(w, "Failed to add user to database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func LoginAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var user struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	database, err := db.Connect("data.db")
	if err != nil {
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}
	defer database.Close()

	hashedPassword, err := database.GetUserPassword(user.Login)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(user.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["name"] = user.Login
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	tokenString, err := token.SignedString(hmacSampleSecret)
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(tokenString))
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("templates/base.html", "templates/register.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userLogin, isAuth := getUserLogin(r)
		data := struct {
			Title     string
			IsAuth    bool
			UserLogin string
		}{
			Title:     "Регистрация",
			IsAuth:    isAuth,
			UserLogin: userLogin,
		}
		err = tmpl.ExecuteTemplate(w, "base.html", data)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.Method == "POST" {
		// Get username and password from the request
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Hash the password
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 8)

		// Connect to the database
		database, _ := db.Connect("data.db")

		// Store the username and hashed password in the database
		err := database.AddUser(username, string(hashedPassword))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Close the database connection
		database.Close()

		// Redirect the user to the login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("templates/base.html", "templates/login.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userLogin, isAuth := getUserLogin(r)
		data := struct {
			Title     string
			IsAuth    bool
			UserLogin string
		}{
			Title:     "Вход",
			IsAuth:    isAuth,
			UserLogin: userLogin,
		}
		err = tmpl.ExecuteTemplate(w, "base.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.Method == "POST" {
		// Get username and password from the request
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Connect to the database
		database, _ := db.Connect("data.db")

		// Get the hashed password of the user from the database
		hashedPassword, err := database.GetUserPassword(username)

		// Compare the stored hashed password, with the hashed version of the password that was received
		if err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
			// If the two passwords don't match, return a 401 status
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// If the passwords match, create a new token for the user
		now := time.Now()
		claims := jwt.MapClaims{
			"name": username,
			"nbf":  now.Unix(),
			"exp":  now.Add(5 * time.Minute).Unix(),
			"iat":  now.Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(hmacSampleSecret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Finally, we set the client cookie for "token" as the JWT we just generated
		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Value:   tokenString,
			Expires: now.Add(5 * time.Minute),
		})

		// Close the database connection
		database.Close()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Redirect(w, r, "/register", http.StatusSeeOther)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tokenStr := c.Value

		tokenFromString, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return hmacSampleSecret, nil
		})

		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		if _, ok := tokenFromString.Claims.(jwt.MapClaims); ok && tokenFromString.Valid {

		} else {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Удалите куки, установив истекшую дату
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   "",
		Expires: time.Unix(0, 0),
	})

	// Перенаправьте пользователя на страницу входа
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

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
	userLogin, isAuth := getUserLogin(r)
	data := struct {
		Title     string
		IsAuth    bool
		UserLogin string
	}{
		Title:     "Добавить выражение",
		IsAuth:    isAuth,
		UserLogin: userLogin,
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
			userLogin, _ := getUserLogin(r)
			userId, _ := database.GetUserID(userLogin)
			id, err = database.AddEquation(id, text, "Equations", userId)
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

	// Get the user id from the JWT token
	userLogin, _ := getUserLogin(r)
	userId, _ := database.GetUserID(userLogin)

	// Get the user_id of the equation from the database
	equationUserId, err := database.GetEquationUserId(id)
	if err != nil {
		// Send an HTTP 404 error for equation not found
		http.Error(w, "Equation not found", http.StatusNotFound)
		return
	}

	// Compare the user id from the JWT token with the user_id of the equation
	if userId != equationUserId {
		// Send an HTTP 401 error for Unauthorized
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the equation with the given id
	equation, status, result, _ := database.GetEquationInfo(id)
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
	values, err = database.GetEquationByUser(getUserLogin(r))
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
	userLogin, isAuth := getUserLogin(r)
	data := struct {
		Title     string
		Equations []map[string]interface{}
		IsAuth    bool
		UserLogin string
	}{
		Title:     "Выражения",
		Equations: values,
		IsAuth:    isAuth,
		UserLogin: userLogin,
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
	userLogin, isAuth := getUserLogin(r)
	data := struct {
		Title      string
		Operations []map[string]interface{}
		IsAuth     bool
		UserLogin  string
	}{
		Title:      "Операции",
		Operations: values,
		IsAuth:     isAuth,
		UserLogin:  userLogin,
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
	userLogin, isAuth := getUserLogin(r)
	data := struct {
		Title     string
		Computers []map[string]interface{}
		IsAuth    bool
		UserLogin string
	}{
		Title:     "Вычислители",
		Computers: values,
		IsAuth:    isAuth,
		UserLogin: userLogin,
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

func getUserLogin(r *http.Request) (string, bool) {
	c, err := r.Cookie("token")
	if err != nil {
		return "", false
	}

	tokenStr := c.Value

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return hmacSampleSecret, nil
	})

	if err != nil {
		return "", false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims["name"].(string), true
	} else {
		return "", false
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
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)

	http.Handle("/", AuthMiddleware(http.HandlerFunc(indexHandler)))
	http.Handle("/add_equation", AuthMiddleware(http.HandlerFunc(addEquationHandler)))
	http.Handle("/get/", AuthMiddleware(http.HandlerFunc(getEquationHandler)))
	http.Handle("/equations", AuthMiddleware(http.HandlerFunc(equationsHandler)))
	http.Handle("/operations", http.HandlerFunc(operationsHandler))
	http.Handle("/computers", http.HandlerFunc(computersHandler))
	http.Handle("/update_operations", http.HandlerFunc(updateOperationsHandler))
	http.Handle("/add_computer", http.HandlerFunc(addComputerHandler))
	http.HandleFunc("/api/v1/register", RegisterAPIHandler)
	http.HandleFunc("/api/v1/login", LoginAPIHandler)

	// Start the HTTP server
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
