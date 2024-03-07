package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var tpl *template.Template

type User struct {
	ID       string    `json:"id,omitempty" bson:"_id,omitempty"`
	Username string    `json:"username,omitempty" bson:"username,omitempty"`
	Email    string    `json:"email,omitempty" bson:"email,omitempty"`
	Password string    `json:"-" bson:"password,omitempty"`
	Created  time.Time `json:"created,omitempty" bson:"created,omitempty"`
}

func init() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, _ = mongo.Connect(context.TODO(), clientOptions)

	err := client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB!")

	router := mux.NewRouter()

	router.HandleFunc("/", homeHandler).Methods("GET")
	router.HandleFunc("/user/{id}", getUserHandler).Methods("GET")
	router.HandleFunc("/register", registerHandler).Methods("POST")
	router.HandleFunc("/delete/{id}", deleteHandler).Methods("DELETE")
	router.HandleFunc("/admin", adminHandler).Methods("GET")
	router.HandleFunc("/admin/add", adminAddHandler).Methods("GET")
	router.HandleFunc("/admin/add", adminAddUserHandler).Methods("POST")
	router.HandleFunc("/admin/edit/{id}", adminEditHandler).Methods("GET")
	router.HandleFunc("/admin/edit/{id}", adminEditUserHandler).Methods("PUT")

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.Handle("/", router)
	tpl = template.Must(template.ParseGlob("templates/*.html"))
}
func adminAddHandler(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFiles("templates/admin_add.html")
	if err != nil {
		log.Fatal(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tpl.Execute(w, nil)
}
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]
	fmt.Println("Deleting user with ID:", id)

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	collection := client.Database("crud").Collection("users")
	_, err = collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
func adminAddUserHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	json.NewDecoder(r.Body).Decode(&user)

	collection := client.Database("crud").Collection("users")
	_, err := collection.InsertOne(context.TODO(), user)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
func adminHandler(w http.ResponseWriter, r *http.Request) {
	users, err := getAllUsers()
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tpl, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		log.Fatal(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tpl.Execute(w, struct{ Users []User }{users})
}
func getAllUsers() ([]User, error) {
	var users []User

	collection := client.Database("crud").Collection("users")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}
func homeHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "index.html", nil)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	json.NewDecoder(r.Body).Decode(&user)

	collection := client.Database("crud").Collection("users")
	user.Created = time.Now()

	_, err := collection.InsertOne(context.TODO(), user)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
func adminEditHandler(w http.ResponseWriter, r *http.Request) {
	tpl, err := template.ParseFiles("templates/admin_edit.html")
	if err != nil {
		log.Fatal(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	params := mux.Vars(r)
	userId := params["id"]
	user, err := getUserById(userId)
	if err != nil {
		log.Println(err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	tpl.Execute(w, user)
}
func adminEditUserHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	var userUpdate User
	if err := json.NewDecoder(r.Body).Decode(&userUpdate); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	collection := client.Database("crud").Collection("users")
	filter := bson.M{"_id": objID}
	update := bson.D{
		{"$set", bson.D{
			{"username", userUpdate.Username},
			{"email", userUpdate.Email},
		}},
	}
	_, err = collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func getUserHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]

	user, err := getUserById(id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
func getUserById(id string) (*User, error) {
	var user User
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	collection := client.Database("crud").Collection("users")
	err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func main() {
	log.Fatal(http.ListenAndServe(":8080", nil))
}
