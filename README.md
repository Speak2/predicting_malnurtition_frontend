# **Malnutrition Prediction Web Application**

## **📌 Project Overview**
This is a **modern and interactive web application** built with **Go**, **HTMX**, and **Tailwind CSS** to predict malnutrition status in children. Users input key metrics such as **age, height, and weight**, and the system calculates **MUAC, HAZ, and WHZ** to determine if the child is **malnourished, normal, or overweight**.

## **🚀 Features**
- 🖥 **User-friendly web interface** with Tailwind CSS.
- ⚡ **Dynamic updates** using HTMX (no page reloads required).
- 🎯 **Real-time malnutrition prediction** based on MUAC, HAZ, and WHZ.
- 💡 **Lightweight and fast backend** using Go's standard `net/http` package.

---

## **📁 Project Structure**
```
malnutrition-prediction/
├── templates/
│   ├── index.html  # Main user interface
├── main.go         # Go backend (handles requests and calculations)
├── go.mod          # Go module file
```

---

## **🛠 Installation and Setup**

### **1️⃣ Install Go (if not installed)**
Download and install Go from [golang.org/dl](https://golang.org/dl/).

### **2️⃣ Clone the Repository**
```sh
 git clone https://github.com/yourusername/malnutrition-prediction.git
 cd malnutrition-prediction
```

### **3️⃣ Initialize Go Module**
```sh
go mod init github.com/yourusername/malnutrition-prediction
```

### **4️⃣ Run the Application**
```sh
go run main.go
```

### **5️⃣ Open the Web App**
Visit **[http://localhost:8080](http://localhost:8080)** in your browser.

---

## **📜 How It Works**

### **Frontend (`index.html`)**
- Users input **age, height, and weight**.
- When the **"Predict"** button is clicked, HTMX sends a **POST request** to the `/predict` endpoint.
- The result is dynamically inserted **without a page reload**.

### **Backend (`main.go`)**
- **Processes form data** from the user.
- Calculates **MUAC, HAZ, and WHZ** based on user inputs.
- Returns **a structured HTML response** with the prediction.

---

## **🔢 Calculation Logic**
The Go backend computes the following:
- **MUAC** = weight / height
- **HAZ** = height / age
- **WHZ** = weight / (height * height)
- **Status Classification**:
  - `WHZ < -2` → **Malnourished**
  - `-2 ≤ WHZ < 2` → **Normal**
  - `WHZ ≥ 2` → **Overweight**

---

## **🎯 API Endpoints**
| Route        | Method | Description |
|-------------|--------|-------------|
| `/`         | GET    | Serves the main webpage |
| `/predict`  | POST   | Processes user input and returns prediction |

---

## **🎨 Technologies Used**
- **Go (`net/http`)** - Backend server
- **HTMX** - Dynamic updates without JavaScript
- **Tailwind CSS** - Modern styling
- **HTML5** - User interface

---

## **🌍 Deployment (Optional)**
You can deploy this project using **Render**, **Fly.io**, or any Go-supported cloud service.

Example: Deploy on **Fly.io**
```sh
flyctl launch
flyctl deploy
```

---

## **📞 Contact & Contributions**
Feel free to contribute to this project by submitting a pull request!
- **Author:** Your Name
- **Email:** your.email@example.com
- **GitHub:** [https://github.com/yourusername](https://github.com/yourusername)

---

### **🚀 Happy Coding!** 🎉

