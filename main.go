package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"github.com/xuri/excelize/v2"
)

// LMS parameters for z-score calculations
type LMS struct {
	L, M, S float64
}

// Data structures to store LMS parameters
var (
	wazData = make(map[string]map[int]LMS)
	hazData = make(map[string]map[int]LMS)
	whzData = make(map[string]map[string]map[float64]LMS)
)


// Load all reference data from Excel files
func loadData() {
	for _, gender := range []string{"M", "F"} {
		wazData[gender] = make(map[int]LMS)
		hazData[gender] = make(map[int]LMS)
		whzData[gender] = map[string]map[float64]LMS{
			"0_2": make(map[float64]LMS),
			"2_5": make(map[float64]LMS),
		}
	}

	loadFile := func(filename string, dataHandler func(rows [][]string)) {
		f, _ := excelize.OpenFile(filename)
		defer f.Close()

		rows, _ := f.GetRows(f.GetSheetList()[0])
		dataHandler(rows[1:]) // Skip header
	}

	parseLMS := func(row []string) (key interface{}, lms LMS, ok bool) {
		if len(row) < 4 {
			return nil, LMS{}, false
		}
		l, _ := strconv.ParseFloat(row[1], 64)
		m, _ := strconv.ParseFloat(row[2], 64)
		s, _ := strconv.ParseFloat(row[3], 64)
		return nil, LMS{l, m, s}, true
	}

	// Load WAZ and HAZ data
	for _, gender := range []string{"M", "F"} {
		for _, dataType := range []struct {
			prefix string
			data   map[string]map[int]LMS
		}{
			{"WAZ", wazData},
			{"HAZ", hazData},
		} {
			loadFile("dataset/"+dataType.prefix+"_"+gender+".xlsx", func(rows [][]string) {
				for _, row := range rows {
					_, lms, ok := parseLMS(row)
					if !ok {
						continue
					}
					age, _ := strconv.Atoi(row[0])
					dataType.data[gender][age] = lms
				}
			})
		}

		// Load WHZ data
		for _, ageRange := range []string{"0-2", "2-5"} {
			mapKey := strings.Replace(ageRange, "-", "_", 1)
			loadFile("dataset/WHZ_"+gender+"_"+ageRange+".xlsx", func(rows [][]string) {
				for _, row := range rows {
					_, lms, ok := parseLMS(row)
					if !ok {
						continue
					}
					height, _ := strconv.ParseFloat(row[0], 64)
					whzData[gender][mapKey][height] = lms
				}
			})
		}
	}
}

// Calculate z-score using the LMS method
func calculateZ(value float64, params LMS) float64 {
	if params.L == 0 {
		return math.Log(value/params.M) / params.S
	}
	return (math.Pow(value/params.M, params.L) - 1) / (params.L * params.S)
}

// Calculate anthropometric indicators and nutritional status
func assess(age, height, weight float64, gender string) (haz, waz, whz float64, status string) {

	// Calculate WAZ
	if data, ok := wazData[gender][int(math.Floor(age))]; ok {
		waz = calculateZ(weight, data)
	}

	// Calculate HAZ
	if data, ok := hazData[gender][int(math.Floor(age))]; ok {
		haz = calculateZ(height, data)
	}

	// Calculate WHZ
	ageRange := "0_2"
	if age > 24 {
		ageRange = "2_5"
	}
	roundedHeight := math.Round(height*10) / 10
	if data, ok := whzData[gender][ageRange][roundedHeight]; ok {
		whz = calculateZ(weight, data)
	}

	// Determine nutritional status
	var conditions []string
	if haz < -2 {
		conditions = append(conditions, "Stunted")
	}
	if whz < -2 {
		conditions = append(conditions, "Wasted")
	}
	if whz > 2 {
		conditions = append(conditions, "Overweight")
	}
	if waz < -2 {
		conditions = append(conditions, "Underweight")
	}

	status = "Normal"
	if len(conditions) > 0 {
		status = strings.Join(conditions, " & ")
	}

	return
}

func main() {
	// Load all reference data
	loadData()

	// Home page handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, _ := template.ParseFiles("templates/index.html")
		tmpl.Execute(w, nil)
	})

	// Assessment handler
	http.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		// Parse input values
		age, _ := strconv.ParseFloat(r.FormValue("age"), 64)
		height, _ := strconv.ParseFloat(r.FormValue("height"), 64)
		weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
		gender := r.FormValue("gender")

		// Calculate metrics
		haz, waz, whz, status := assess(age, height, weight, gender)

		// Set status color
		statusColor := "text-red-500"
		if status == "Normal" {
			statusColor = "text-green-500"
		} else if strings.Contains(status, "At Risk") {
			statusColor = "text-yellow-500"
		}

		// Send response
		fmt.Fprintf(w, `<div class="p-6 border rounded-lg bg-gray-50 shadow-sm">
			<h3 class="text-xl font-bold mb-3 text-blue-600">Assessment Results</h3>
			<div class="space-y-2">
				<p class="flex justify-between border-b pb-2">
					<span class="font-medium">Height-for-Age Z-score (HAZ):</span>
					<span class="font-bold">%.2f</span>
				</p>
				<p class="flex justify-between border-b pb-2">
					<span class="font-medium">Weight-for-Age Z-score (WAZ):</span>
					<span class="font-bold">%.2f</span>
				</p>
				<p class="flex justify-between border-b pb-2">
					<span class="font-medium">Weight-for-Height Z-score (WHZ):</span>
					<span class="font-bold">%.2f</span>
				</p>
				<p class="flex justify-between pt-1">
					<span class="font-medium">Nutritional Status:</span>
					<span class="font-bold %s">%s</span>
				</p>
			</div>
		</div>`, haz, waz, whz, statusColor, status)
	})

	// Start server
	fmt.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}