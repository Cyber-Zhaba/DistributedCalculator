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
	log.Println("Index handler")
	tmpl, err := template.ParseFiles("templates\\base.html", "templates\\index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	data := struct {
		Title string
	}{
		Title: "Добавить выражение",
	}

	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func addEquationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		log.Println("addEquationHandler")
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Fatal(err)
			return
		}
		idStr := r.FormValue("id")
		text := r.FormValue("text")
		fmt.Println(text)

		database, err := db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
			return
		}
		defer func() {
			if err = database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		id := 0
		if idStr != "" {
			id, err = strconv.Atoi(idStr)
			http.Redirect(w, r, fmt.Sprintf("/get/%d", id), http.StatusSeeOther)
		} else {
			// Check if the equation is valid
			if !agent.ValidEquation(text, 0, len(text)) {
				http.Error(w, "Invalid equation", http.StatusBadRequest)
				log.Println("Invalid equation")
				return
			}
			id, err = database.AddEquation(id, text, "Equations")
			if err != nil {
				log.Fatal(err)
			}

			go func() {
				err = agent.Evaluate(id)
				if err != nil {
					log.Fatal(err)
				}
			}()

			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	}
}

func getEquationHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the URL path to get the id
	path := strings.Split(r.URL.Path, "/")
	idStr := path[len(path)-1]

	// Convert the id to an integer
	id, err := strconv.Atoi(idStr)
	if err != nil {
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
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Get the equation with the given id
	equation, status, result := database.GetEquationInfo(id)
	if err != nil {
		http.Error(w, "Equation not found", http.StatusNotFound)
		return
	}
	var jsonStr []byte
	jsonStr, err = json.Marshal(map[string]interface{}{
		"id":     id,
		"text":   equation,
		"status": status,
		"result": result,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonStr)
	if err != nil {
		log.Fatal(err)
	}
}

func equationsHandler(w http.ResponseWriter, r *http.Request) {
	database, err := db.Connect("data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	var values []map[string]interface{}
	values, err = database.GetAllValues("Equations")
	if err != nil {
		log.Fatal(err)
	}

	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates\\base.html", "templates\\equations.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	data := struct {
		Title     string
		Equations []map[string]interface{}
	}{
		Title:     "Выражения",
		Equations: values,
	}

	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func operationsHandler(w http.ResponseWriter, r *http.Request) {
	// All operations
	database, err := db.Connect("data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	var values []map[string]interface{}
	values, err = database.GetAllValues("Operations")
	if err != nil {
		log.Fatal(err)
	}

	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates\\base.html", "templates\\operations.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	data := struct {
		Title      string
		Operations []map[string]interface{}
	}{
		Title:      "Операции",
		Operations: values,
	}

	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func updateOperationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		plusTime := r.FormValue("time_+")
		minusTime := r.FormValue("time_-")
		multiplyTime := r.FormValue("time_*")
		divideTime := r.FormValue("time_/")

		database, err := db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		types := []string{"+", "-", "*", "/"}
		times := []string{plusTime, minusTime, multiplyTime, divideTime}

		err = database.UpdateOperations(types, times)
		if err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/operations", http.StatusSeeOther)
	}
}

func computersHandler(w http.ResponseWriter, r *http.Request) {
	// All computers
	database, err := db.Connect("data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = database.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	var values []map[string]interface{}
	values, err = database.GetAllValues("Computers")
	if err != nil {
		log.Fatal(err)
	}

	var tmpl *template.Template
	tmpl, err = template.ParseFiles("templates\\base.html", "templates\\computers.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	data := struct {
		Title     string
		Computers []map[string]interface{}
	}{
		Title:     "Вычислители",
		Computers: values,
	}

	err = tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
		return
	}
}

func addComputerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Fatal(err)
			return
		}

		var database *db.DB
		database, err = db.Connect("data.db")
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err = database.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		err = database.AddComputer()
		if err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/computers", http.StatusSeeOther)
	}
}

func main() {
	// Check if database exists and create it if it doesn't
	_, err := os.Stat("data.db")
	if os.IsNotExist(err) {
		var file *os.File
		file, err = os.Create("data.db")
		if err != nil {
			fmt.Println(err)
		}
		err = file.Close()
		if err != nil {
			fmt.Println(err)
		}
	}
	database, _ := db.Connect("data.db")
	err = database.Init()
	if err != nil {
		fmt.Println(err)
	}
	err = database.Close()
	if err != nil {
		fmt.Println(err)
	}

	logFile, err := os.OpenFile(".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
	}
	log.SetOutput(logFile)
	log.Println("Server started")

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/add_equation", addEquationHandler)
	http.HandleFunc("/get/", getEquationHandler)
	http.HandleFunc("/equations", equationsHandler)
	http.HandleFunc("/operations", operationsHandler)
	http.HandleFunc("/computers", computersHandler)
	http.HandleFunc("/update_operations", updateOperationsHandler)
	http.HandleFunc("/add_computer", addComputerHandler)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
