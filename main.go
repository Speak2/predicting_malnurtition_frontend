package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Configuration constants and defaults
var (
	defaultPort      = "8080"
	defaultPythonURL = "http://127.0.0.1:8000"
	pythonAPIURL     = defaultPythonURL
)

// Registered model identifiers
var availableModels = []string{
	"logistic_regression",
	"random_forest",
	"gradient_boosting",
	"stacking_ensemble",
	"svm_rbf",
	"mlp",
	"decision_tree",
}

// ModelInfo describes a machine learning model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	File        string `json:"file"`
	Algorithm   string `json:"algorithm"`
	IsLoaded    bool   `json:"is_loaded"`
	Recommended bool   `json:"recommended"`
	Description string `json:"description"`
}

// PredictionRequest matches Predictor Set C feature schema
type PredictionRequest struct {
	Month        float64 `json:"month"`
	SexMale      int     `json:"sex_male"`
	AreaNoakhali int     `json:"area_noakhali"`
	Weight       float64 `json:"weight"`
	Height       float64 `json:"height"`
	WAZ          float64 `json:"waz"`
	HAZ          float64 `json:"haz"`
	WHZ          float64 `json:"whz"`
	ModelName    string  `json:"model_name,omitempty"`
}

// PredictionResponse represents the backend ML prediction output
type PredictionResponse struct {
	ModelUsed       string             `json:"model_used"`
	ModelFile       string             `json:"model_file"`
	ModelType       string             `json:"model_type"`
	Prediction      int                `json:"prediction"`
	Status          string             `json:"status"`
	RiskProbability float64            `json:"risk_probability"`
	RiskLevel       string             `json:"risk_level"`
	ModelLoaded     bool               `json:"model_loaded"`
	FeaturesUsed    []float64          `json:"features_used"`
	FeatureDict     map[string]float64 `json:"feature_dict"`
	LatencyMs       int64              `json:"latency_ms,omitempty"`
}

// MultiModelPredictionResponse aggregates predictions across all 7 models
type MultiModelPredictionResponse struct {
	ConsensusStatus    string                        `json:"consensus_status"`
	MalnourishedCount  int                           `json:"malnourished_count"`
	NormalCount        int                           `json:"normal_count"`
	ConsensusAgreement float64                       `json:"consensus_agreement"`
	MeanRisk           float64                       `json:"mean_risk"`
	OverallRiskLevel   string                        `json:"overall_risk_level"`
	Results            map[string]PredictionResponse `json:"results"`
	LatencyMs          int64                         `json:"latency_ms"`
}

// HealthResponse reflects health state of both Go frontend and Python backend
type HealthResponse struct {
	Status        string                 `json:"status"`
	Timestamp     string                 `json:"timestamp"`
	Frontend      string                 `json:"frontend"`
	PythonBackend map[string]interface{} `json:"python_backend"`
}

// EstimateZScoresRequest for on-the-fly WHO calculations
type EstimateZScoresRequest struct {
	Month   float64 `json:"month"`
	SexMale int     `json:"sex_male"`
	Weight  float64 `json:"weight"`
	Height  float64 `json:"height"`
}

type EstimateZScoresResponse struct {
	WAZ float64 `json:"waz"`
	HAZ float64 `json:"haz"`
	WHZ float64 `json:"whz"`
}

// WHO 2006 Reference parameter table structures
type LMS struct {
	L, M, S float64
}

// calculateZ computes LMS z-score
func calculateZ(y, l, m, s float64) float64 {
	if s <= 0 || m <= 0 || y <= 0 {
		return 0.0
	}
	if math.Abs(l) < 0.0001 {
		return math.Log(y/m) / s
	}
	return (math.Pow(y/m, l) - 1.0) / (l * s)
}

// EstimateWHOZScores calculates realistic WHO 2006 standard Z-scores
func EstimateWHOZScores(month float64, isMale bool, weight, height float64) (waz, haz, whz float64) {
	// Clamp values
	m := math.Max(0.0, math.Min(60.0, month))

	// Approximate WHO median growth parameters across 0-60 months
	var medHeight, medWeight float64
	var hazS, wazS, wazL float64

	if isMale {
		// Boys Height-for-age: ~49.9cm at birth to ~110.0cm at 60m
		medHeight = 49.9 + 2.15*math.Pow(m, 0.82)
		hazS = 0.038
		// Boys Weight-for-age: ~3.3kg at birth to ~18.3kg at 60m
		medWeight = 3.3 + 0.95*math.Pow(m, 0.68)
		wazS = 0.12
		wazL = -0.15
	} else {
		// Girls Height-for-age: ~49.1cm at birth to ~109.4cm at 60m
		medHeight = 49.1 + 2.10*math.Pow(m, 0.82)
		hazS = 0.039
		// Girls Weight-for-age: ~3.2kg at birth to ~17.9kg at 60m
		medWeight = 3.2 + 0.92*math.Pow(m, 0.68)
		wazS = 0.13
		wazL = -0.18
	}

	// Calculate HAZ (Height-for-Age)
	if height > 0 && medHeight > 0 {
		haz = calculateZ(height, 1.0, medHeight, hazS)
	}

	// Calculate WAZ (Weight-for-Age)
	if weight > 0 && medWeight > 0 {
		waz = calculateZ(weight, wazL, medWeight, wazS)
	}

	// Calculate WHZ (Weight-for-Height)
	if height > 0 && weight > 0 {
		h := math.Max(45.0, math.Min(120.0, height))
		// Median weight for height curve
		var medWeightForHeight float64
		if isMale {
			medWeightForHeight = 2.5 * math.Pow(h/45.0, 2.15)
		} else {
			medWeightForHeight = 2.4 * math.Pow(h/45.0, 2.15)
		}
		whz = calculateZ(weight, -0.35, medWeightForHeight, 0.095)
	}

	// Round to 3 decimal places and clamp to clinical range [-6.0, 6.0]
	clamp := func(v float64) float64 {
		if v < -6.0 {
			return -6.0
		}
		if v > 6.0 {
			return 6.0
		}
		return math.Round(v*1000) / 1000
	}

	return clamp(waz), clamp(haz), clamp(whz)
}

// queryPythonPredict makes a POST request to Python /predict
func queryPythonPredict(ctx context.Context, req PredictionRequest) (*PredictionResponse, error) {
	startTime := time.Now()
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", pythonAPIURL+"/predict", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("python ML service unreachable at %s: %w", pythonAPIURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python ML service returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var predResp PredictionResponse
	if err := json.Unmarshal(bodyBytes, &predResp); err != nil {
		return nil, fmt.Errorf("failed to parse prediction response: %w", err)
	}
	predResp.LatencyMs = time.Since(startTime).Milliseconds()

	return &predResp, nil
}

// queryPythonPredictAll runs inference across all 7 models concurrently
func queryPythonPredictAll(ctx context.Context, baseReq PredictionRequest) (*MultiModelPredictionResponse, error) {
	startTime := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex

	results := make(map[string]PredictionResponse)
	var firstErr error

	for _, modelKey := range availableModels {
		wg.Add(1)
		go func(mKey string) {
			defer wg.Done()
			req := baseReq
			req.ModelName = mKey

			pred, err := queryPythonPredict(ctx, req)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			results[mKey] = *pred
		}(modelKey)
	}

	wg.Wait()

	if len(results) == 0 && firstErr != nil {
		return nil, firstErr
	}

	malnourishedCount := 0
	normalCount := 0
	var sumRisk float64

	for _, res := range results {
		if res.Prediction == 1 {
			malnourishedCount++
		} else {
			normalCount++
		}
		sumRisk += res.RiskProbability
	}

	total := len(results)
	meanRisk := 0.0
	if total > 0 {
		meanRisk = math.Round((sumRisk/float64(total))*1000) / 1000
	}

	consensusStatus := "Normal"
	agreementRate := float64(normalCount) / float64(total)
	if malnourishedCount > normalCount {
		consensusStatus = "Malnourished"
		agreementRate = float64(malnourishedCount) / float64(total)
	}

	riskLevel := "Low Risk"
	if meanRisk >= 0.50 {
		riskLevel = "High Risk"
	} else if meanRisk >= 0.30 {
		riskLevel = "Moderate Risk"
	}

	return &MultiModelPredictionResponse{
		ConsensusStatus:    consensusStatus,
		MalnourishedCount:  malnourishedCount,
		NormalCount:        normalCount,
		ConsensusAgreement: math.Round(agreementRate*1000) / 10,
		MeanRisk:           meanRisk,
		OverallRiskLevel:   riskLevel,
		Results:            results,
		LatencyMs:          time.Since(startTime).Milliseconds(),
	}, nil
}

// checkBackendHealth probes Python /health
func checkBackendHealth(ctx context.Context) HealthResponse {
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", pythonAPIURL+"/health", nil)

	pyData := make(map[string]interface{})
	overallStatus := "healthy"

	resp, err := client.Do(req)
	if err != nil {
		overallStatus = "degraded"
		pyData["status"] = "offline"
		pyData["url"] = pythonAPIURL
		pyData["error"] = fmt.Sprintf("Unreachable: %v", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &pyData); err != nil {
			overallStatus = "degraded"
			pyData["status"] = "invalid_response"
			pyData["error"] = err.Error()
		} else {
			pyData["url"] = pythonAPIURL
		}
	}

	return HealthResponse{
		Status:        overallStatus,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Frontend:      "healthy",
		PythonBackend: pyData,
	}
}

// parsePredictionForm handles form extraction with aliases and auto-zscore fallback
func parsePredictionForm(r *http.Request) (PredictionRequest, bool) {
	r.ParseForm()

	// Parse month / age
	monthStr := r.FormValue("month")
	if monthStr == "" {
		monthStr = r.FormValue("age")
	}
	month, _ := strconv.ParseFloat(monthStr, 64)

	// Parse sex: "M"/"F" or "1"/"0"
	sexMale := 1
	sexStr := strings.TrimSpace(strings.ToUpper(r.FormValue("sex_male")))
	if sexStr == "" {
		sexStr = strings.TrimSpace(strings.ToUpper(r.FormValue("gender")))
	}
	if sexStr == "F" || sexStr == "0" || sexStr == "FEMALE" {
		sexMale = 0
	}

	// Parse district / area: "1" (Noakhali), "0" (Lakshmipur)
	areaNoakhali := 0
	areaStr := strings.TrimSpace(strings.ToLower(r.FormValue("area_noakhali")))
	if areaStr == "" {
		areaStr = strings.TrimSpace(strings.ToLower(r.FormValue("district")))
	}
	if areaStr == "1" || areaStr == "noakhali" {
		areaNoakhali = 1
	}

	weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
	height, _ := strconv.ParseFloat(r.FormValue("height"), 64)

	// Check if Z-scores were provided
	wazStr := r.FormValue("waz")
	hazStr := r.FormValue("haz")
	whzStr := r.FormValue("whz")

	var waz, haz, whz float64
	autoCalculate := r.FormValue("auto_zscore") == "1" || r.FormValue("auto_zscore") == "true" || (wazStr == "" && hazStr == "" && whzStr == "")

	if autoCalculate {
		waz, haz, whz = EstimateWHOZScores(month, sexMale == 1, weight, height)
	} else {
		waz, _ = strconv.ParseFloat(wazStr, 64)
		haz, _ = strconv.ParseFloat(hazStr, 64)
		whz, _ = strconv.ParseFloat(whzStr, 64)
	}

	modelName := strings.TrimSpace(r.FormValue("model_name"))
	if modelName == "" {
		modelName = "logistic_regression"
	}

	compareAll := r.FormValue("compare_all") == "1" || r.FormValue("compare_all") == "true"

	return PredictionRequest{
		Month:        month,
		SexMale:      sexMale,
		AreaNoakhali: areaNoakhali,
		Weight:       weight,
		Height:       height,
		WAZ:          waz,
		HAZ:          haz,
		WHZ:          whz,
		ModelName:    modelName,
	}, compareAll
}

// renderHTMLResult generates the HTML fragment for HTMX / browser view
func renderHTMLResult(w http.ResponseWriter, single *PredictionResponse, multi *MultiModelPredictionResponse, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err != nil {
		fmt.Fprintf(w, `
		<div class="rounded-xl border border-red-200 bg-red-50 p-6 shadow-sm animate-fade-in">
			<div class="flex items-start space-x-3">
				<div class="flex-shrink-0 text-red-500 text-2xl">
					<i class="fas fa-exclamation-triangle"></i>
				</div>
				<div class="flex-1">
					<h3 class="text-lg font-bold text-red-800">Inference Service Unavailable</h3>
					<p class="mt-1 text-sm text-red-700">%s</p>
					<div class="mt-4 rounded-lg bg-white p-4 border border-red-200 text-xs text-gray-700 space-y-2">
						<p class="font-semibold text-gray-900">How to start the Python ML Service:</p>
						<ol class="list-decimal list-inside space-y-1 font-mono text-gray-600">
							<li>Open a terminal in: <span class="bg-gray-100 px-1 py-0.5 rounded">Malnutrition-Prediction-python-service</span></li>
							<li>Run: <span class="text-blue-600 font-bold">run_server.bat</span> (or <span class="text-blue-600 font-bold">uvicorn main:app --reload --port 8000</span>)</li>
							<li>Verify health at: <a href="http://127.0.0.1:8000/health" target="_blank" class="text-blue-600 underline">http://127.0.0.1:8000/health</a></li>
						</ol>
					</div>
				</div>
			</div>
		</div>`, template.HTMLEscapeString(err.Error()))
		return
	}

	if multi != nil {
		// Multi-model consensus view
		statusColor := "bg-rose-500 text-white"
		statusIcon := "fa-triangle-exclamation"
		statusTitle := "ACUTE MALNUTRITION DETECTED"
		if multi.ConsensusStatus == "Normal" {
			statusColor = "bg-emerald-500 text-white"
			statusIcon = "fa-circle-check"
			statusTitle = "NORMAL NUTRITIONAL STATUS"
		}

		rowsHTML := ""
		for _, mKey := range availableModels {
			res, ok := multi.Results[mKey]
			if !ok {
				continue
			}
			badgeClass := "bg-rose-100 text-rose-800 border-rose-200"
			if res.Prediction == 0 {
				badgeClass = "bg-emerald-100 text-emerald-800 border-emerald-200"
			}
			probPercent := int(res.RiskProbability * 100)
			rowsHTML += fmt.Sprintf(`
				<tr class="border-b border-gray-100 hover:bg-gray-50/80 transition">
					<td class="py-3 px-4 font-medium text-gray-900 flex items-center space-x-2">
						<span>%s</span>
					</td>
					<td class="py-3 px-4 text-xs font-mono text-gray-500">%s</td>
					<td class="py-3 px-4">
						<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold border %s">
							%s
						</span>
					</td>
					<td class="py-3 px-4">
						<div class="flex items-center space-x-2">
							<div class="w-24 bg-gray-200 rounded-full h-2">
								<div class="bg-blue-600 h-2 rounded-full" style="width: %d%%"></div>
							</div>
							<span class="text-xs font-bold text-gray-700">%.1f%%</span>
						</div>
					</td>
					<td class="py-3 px-4 text-xs text-gray-500">%s</td>
				</tr>
			`, res.ModelUsed, res.ModelType, badgeClass, res.Status, probPercent, res.RiskProbability*100, res.RiskLevel)
		}

		fmt.Fprintf(w, `
		<div class="space-y-6 animate-fade-in">
			<!-- Consensus Header Banner -->
			<div class="rounded-2xl p-6 shadow-lg %s flex flex-col md:flex-row items-center justify-between gap-4">
				<div class="flex items-center space-x-4">
					<div class="w-14 h-14 rounded-2xl bg-white/20 backdrop-blur-sm flex items-center justify-center text-3xl">
						<i class="fas %s"></i>
					</div>
					<div>
						<span class="text-xs font-bold tracking-wider uppercase opacity-90">Multi-Model Ensemble Consensus</span>
						<h2 class="text-2xl font-black tracking-tight">%s</h2>
						<p class="text-xs opacity-90 mt-0.5">Evaluated across all 7 Set C algorithms in %d ms</p>
					</div>
				</div>
				<div class="bg-white/10 backdrop-blur-md rounded-xl px-5 py-3 text-center border border-white/20">
					<span class="text-xs font-medium uppercase tracking-wider block opacity-80">Model Agreement</span>
					<span class="text-3xl font-black">%.0f%%</span>
					<span class="text-xs block opacity-80">%d of 7 agree</span>
				</div>
			</div>

			<!-- Consensus Metrics Grid -->
			<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
				<div class="bg-white p-5 rounded-xl border border-gray-200 shadow-sm">
					<span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Average Risk Probability</span>
					<div class="mt-2 flex items-baseline space-x-2">
						<span class="text-3xl font-black text-gray-900">%.1f%%</span>
						<span class="text-xs font-semibold px-2 py-0.5 rounded-full %s">%s</span>
					</div>
					<div class="w-full bg-gray-100 rounded-full h-2 mt-3">
						<div class="h-2 rounded-full %s" style="width: %.1f%%"></div>
					</div>
				</div>

				<div class="bg-white p-5 rounded-xl border border-gray-200 shadow-sm">
					<span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Target Clinical Metric</span>
					<div class="mt-2 text-gray-900 font-bold text-lg">Acute Malnutrition</div>
					<p class="text-xs text-gray-500 mt-1">Mid-Upper Arm Circumference (<span class="font-bold text-rose-600">MUAC &lt; 12.5 cm</span>)</p>
				</div>

				<div class="bg-white p-5 rounded-xl border border-gray-200 shadow-sm">
					<span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">Triage Recommendation</span>
					<div class="mt-2 text-sm font-semibold text-gray-800">
						%s
					</div>
				</div>
			</div>

			<!-- Detailed Model Comparison Table -->
			<div class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
				<div class="px-5 py-4 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
					<h3 class="font-bold text-gray-800 flex items-center space-x-2">
						<i class="fas fa-layer-group text-blue-600"></i>
						<span>Individual Model Inference Results (7 Algorithms)</span>
					</h3>
					<span class="text-xs font-medium text-gray-500">Non-circular Set C Predictors</span>
				</div>
				<div class="overflow-x-auto">
					<table class="w-full text-left text-sm text-gray-600">
						<thead class="bg-gray-50/50 text-xs uppercase font-semibold text-gray-500 border-b border-gray-200">
							<tr>
								<th class="py-3 px-4">Model Identifier</th>
								<th class="py-3 px-4">Architecture</th>
								<th class="py-3 px-4">Classification</th>
								<th class="py-3 px-4">Risk Probability</th>
								<th class="py-3 px-4">Clinical Tier</th>
							</tr>
						</thead>
						<tbody>
							%s
						</tbody>
					</table>
				</div>
			</div>
		</div>
		`,
			statusColor, statusIcon, statusTitle, multi.LatencyMs,
			multi.ConsensusAgreement,
			multi.MalnourishedCount,
			multi.MeanRisk*100,
			getRiskBadgeClass(multi.OverallRiskLevel), multi.OverallRiskLevel,
			getRiskBarClass(multi.OverallRiskLevel), multi.MeanRisk*100,
			getClinicalTriageText(multi.ConsensusStatus, multi.MeanRisk),
			rowsHTML,
		)
		return
	}

	if single != nil {
		// Single Model View
		statusColor := "bg-rose-500 text-white"
		statusIcon := "fa-triangle-exclamation"
		statusTitle := "ACUTE MALNUTRITION DETECTED"
		if single.Prediction == 0 {
			statusColor = "bg-emerald-500 text-white"
			statusIcon = "fa-circle-check"
			statusTitle = "NORMAL NUTRITIONAL STATUS"
		}

		probPercent := single.RiskProbability * 100

		fmt.Fprintf(w, `
		<div class="space-y-6 animate-fade-in">
			<!-- Main Result Card -->
			<div class="rounded-2xl p-6 shadow-lg %s flex flex-col md:flex-row items-center justify-between gap-4">
				<div class="flex items-center space-x-4">
					<div class="w-14 h-14 rounded-2xl bg-white/20 backdrop-blur-sm flex items-center justify-center text-3xl">
						<i class="fas %s"></i>
					</div>
					<div>
						<span class="text-xs font-bold tracking-wider uppercase opacity-90">%s Inference</span>
						<h2 class="text-2xl font-black tracking-tight">%s</h2>
						<p class="text-xs opacity-90 mt-0.5">Target: MUAC &lt; 12.5 cm | Pipeline: %s</p>
					</div>
				</div>
				<div class="bg-white/10 backdrop-blur-md rounded-xl px-5 py-3 text-center border border-white/20">
					<span class="text-xs font-medium uppercase tracking-wider block opacity-80">Predicted Probability</span>
					<span class="text-3xl font-black">%.1f%%</span>
					<span class="text-xs font-semibold px-2 py-0.5 rounded-full inline-block mt-1 %s">%s</span>
				</div>
			</div>

			<!-- Metrics & Clinical Recommendation -->
			<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div class="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-4">
					<h3 class="font-bold text-gray-800 flex items-center space-x-2 text-sm">
						<i class="fas fa-stethoscope text-blue-600"></i>
						<span>Clinical Triage Recommendation</span>
					</h3>
					<div class="text-sm text-gray-700 bg-blue-50/50 p-4 rounded-xl border border-blue-100">
						%s
					</div>
					<div class="text-xs text-gray-500">
						<span class="font-semibold text-gray-700">Clinical Protocol:</span> Based on WHO Child Growth Standards &amp; CMAM acute malnutrition management guidelines.
					</div>
				</div>

				<div class="bg-white p-5 rounded-xl border border-gray-200 shadow-sm space-y-3">
					<h3 class="font-bold text-gray-800 flex items-center space-x-2 text-sm">
						<i class="fas fa-sliders text-indigo-600"></i>
						<span>Predictor Vector (Set C Inputs)</span>
					</h3>
					<div class="grid grid-cols-2 gap-2 text-xs">
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">Age:</span> <span class="font-bold text-gray-800">%.1f mo</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">Weight:</span> <span class="font-bold text-gray-800">%.2f kg</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">Height:</span> <span class="font-bold text-gray-800">%.1f cm</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">Sex:</span> <span class="font-bold text-gray-800">%s</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">WAZ:</span> <span class="font-bold text-gray-800">%.2f</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">HAZ:</span> <span class="font-bold text-gray-800">%.2f</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">WHZ:</span> <span class="font-bold text-gray-800">%.2f</span></div>
						<div class="p-2 bg-gray-50 rounded-lg"><span class="text-gray-500">Area:</span> <span class="font-bold text-gray-800">%s</span></div>
					</div>
					<div class="pt-2 text-xs text-gray-400 flex justify-between">
						<span>Inference Latency: <strong>%d ms</strong></span>
						<span>Model: <strong>%s</strong></span>
					</div>
				</div>
			</div>
		</div>
		`,
			statusColor, statusIcon, single.ModelUsed, statusTitle, single.ModelType,
			probPercent, getRiskBadgeClass(single.RiskLevel), single.RiskLevel,
			getClinicalTriageText(single.Status, single.RiskProbability),
			single.FeatureDict["month"],
			single.FeatureDict["weight"],
			single.FeatureDict["height"],
			formatSex(single.FeatureDict["sex_male"]),
			single.FeatureDict["waz"],
			single.FeatureDict["haz"],
			single.FeatureDict["whz"],
			formatArea(single.FeatureDict["area_noakhali"]),
			single.LatencyMs,
			single.ModelFile,
		)
	}
}

func getRiskBadgeClass(level string) string {
	switch level {
	case "High Risk":
		return "bg-rose-100 text-rose-800 border-rose-300"
	case "Moderate Risk":
		return "bg-amber-100 text-amber-800 border-amber-300"
	default:
		return "bg-emerald-100 text-emerald-800 border-emerald-300"
	}
}

func getRiskBarClass(level string) string {
	switch level {
	case "High Risk":
		return "bg-rose-500"
	case "Moderate Risk":
		return "bg-amber-500"
	default:
		return "bg-emerald-500"
	}
}

func formatSex(v float64) string {
	if v == 1 {
		return "Male"
	}
	return "Female"
}

func formatArea(v float64) string {
	if v == 1 {
		return "Noakhali"
	}
	return "Lakshmipur"
}

func getClinicalTriageText(status string, prob float64) string {
	if status == "Malnourished" || prob >= 0.50 {
		return `<strong>Severe / Moderate Acute Malnutrition Detected.</strong><br>
		1. Conduct physical exam for bilateral pitting edema.<br>
		2. Perform an appetite test using Ready-to-Use Therapeutic Food (RUTF).<br>
		3. If appetite test is passed and child has no medical complications, initiate outpatient CMAM protocol.<br>
		4. If appetite test is failed or severe edema/complications exist, execute urgent inpatient referral.`
	} else if prob >= 0.30 {
		return `<strong>Borderline / At-Risk Nutritional State.</strong><br>
		1. Enroll in targeted supplementary feeding or community nutrition counseling.<br>
		2. Review maternal infant feeding practices, dietary diversity, and micronutrient supplementation.<br>
		3. Schedule follow-up anthropometric screening within 14 days to monitor growth faltering.`
	}
	return `<strong>Normal Nutritional Status.</strong><br>
	1. Child metrics align with healthy cohort thresholds (MUAC ≥ 12.5 cm).<br>
	2. Continue standard age-appropriate infant and young child feeding (IYCF) counseling.<br>
	3. Recommend next routine growth monitoring at scheduled vaccination / health post visit.`
}

func main() {
	// Support environment variables
	if envPort := os.Getenv("PORT"); envPort != "" {
		defaultPort = envPort
	}
	if envPyURL := os.Getenv("PYTHON_API_URL"); envPyURL != "" {
		pythonAPIURL = strings.TrimRight(envPyURL, "/")
	}

	log.Printf("Starting Malnutrition Prediction Web Application...")
	log.Printf("Configured Python Backend URL: %s", pythonAPIURL)

	// Static web interface handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			http.Error(w, fmt.Sprintf("Template rendering error: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, map[string]interface{}{
			"PythonAPIURL": pythonAPIURL,
			"Models":       availableModels,
		})
	})

	// Unified prediction endpoint (handles both HTMX forms and JSON)
	http.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		req, compareAll := parsePredictionForm(r)
		isHTMX := r.Header.Get("HX-Request") == "true"
		acceptJSON := strings.Contains(r.Header.Get("Accept"), "application/json")

		if compareAll {
			multiResp, err := queryPythonPredictAll(r.Context(), req)
			if acceptJSON && !isHTMX {
				w.Header().Set("Content-Type", "application/json")
				if err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				json.NewEncoder(w).Encode(multiResp)
				return
			}
			renderHTMLResult(w, nil, multiResp, err)
			return
		}

		singleResp, err := queryPythonPredict(r.Context(), req)
		if acceptJSON && !isHTMX {
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(singleResp)
			return
		}
		renderHTMLResult(w, singleResp, nil, err)
	})

	// JSON API: /api/predict
	http.HandleFunc("/api/predict", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
			return
		}

		var req PredictionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Fallback to form parsing
			req, _ = parsePredictionForm(r)
		}

		resp, err := queryPythonPredict(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// JSON API: /api/predict-all (Multi-model consensus)
	http.HandleFunc("/api/predict-all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
			return
		}

		var req PredictionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req, _ = parsePredictionForm(r)
		}

		resp, err := queryPythonPredictAll(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Health check endpoint (probes both frontend and Python backend)
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		health := checkBackendHealth(r.Context())
		if health.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(health)
	})

	// Models catalog proxy
	http.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(pythonAPIURL + "/models")
		if err != nil {
			// Provide fallback models catalog if python is offline
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "fallback",
				"total_models":  len(availableModels),
				"default_model": "logistic_regression",
				"models": []map[string]interface{}{
					{"id": "logistic_regression", "name": "Logistic Regression (Set C Pipeline)", "recommended": true},
					{"id": "random_forest", "name": "Random Forest Classifier", "recommended": false},
					{"id": "gradient_boosting", "name": "Gradient Boosting Classifier", "recommended": false},
					{"id": "stacking_ensemble", "name": "Stacking Ensemble Classifier", "recommended": false},
					{"id": "svm_rbf", "name": "Support Vector Machine (RBF Kernel)", "recommended": false},
					{"id": "mlp", "name": "Multi-Layer Perceptron (Neural Network)", "recommended": false},
					{"id": "decision_tree", "name": "Decision Tree Classifier", "recommended": false},
				},
			})
			return
		}
		defer resp.Body.Close()
		io.Copy(w, resp.Body)
	})

	// WHO Z-Score Estimation API
	http.HandleFunc("/api/estimate-zscores", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
			return
		}

		var req EstimateZScoresRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			month, _ := strconv.ParseFloat(r.FormValue("month"), 64)
			sexMale, _ := strconv.Atoi(r.FormValue("sex_male"))
			weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
			height, _ := strconv.ParseFloat(r.FormValue("height"), 64)
			req = EstimateZScoresRequest{
				Month:   month,
				SexMale: sexMale,
				Weight:  weight,
				Height:  height,
			}
		}

		waz, haz, whz := EstimateWHOZScores(req.Month, req.SexMale == 1, req.Weight, req.Height)
		json.NewEncoder(w).Encode(EstimateZScoresResponse{
			WAZ: waz,
			HAZ: haz,
			WHZ: whz,
		})
	})

	addr := ":" + defaultPort
	log.Printf("🚀 Web frontend server listening at http://localhost%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
