package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    // 1. Define the route and the handler function
    http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        
        // 2. Reject anything that isn't a POST request
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        // 3. Set the response header to JSON
        w.Header().Set("Content-Type", "application/json")
        
        // 4. Send back a simple JSON dictionary
        response := map[string]string{"status": "File received!"}
        json.NewEncoder(w).Encode(response)
    })

    // Start the server on port 8080
    fmt.Println("Server is running on http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}