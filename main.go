package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type party struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	GSTNumber string `json:"gstNumber"`
}

type consignmentItem struct {
	Quantity      string `json:"quantity"`
	Description   string `json:"description"`
	Weight        string `json:"weight"`
	Rate          string `json:"rate"`
	Amount        string `json:"amount"`
	Payment       string `json:"payment"`
	InvoiceNumber string `json:"invoiceNumber"`
}

type charges struct {
	RTO                    string `json:"rto"`
	AdditionalOtherCharges string `json:"additionalOtherCharges"`
	Hamali                 string `json:"hamali"`
	Other                  string `json:"other"`
	Door                   string `json:"door"`
	Station                string `json:"station"`
}

type transporter struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	AlternatePhone   string `json:"alternatePhone"`
	EnrollmentNumber string `json:"enrollmentNumber"`
	Jurisdiction     string `json:"jurisdiction"`
}

type consignment struct {
	ID                    string            `json:"id"`
	Transporter           transporter       `json:"transporter"`
	ConsignmentNumber     string            `json:"consignmentNumber"`
	Date                  string            `json:"date"`
	CopyType              string            `json:"copyType"`
	InsuranceStatus       string            `json:"insuranceStatus"`
	InsuranceCompany      string            `json:"insuranceCompany"`
	InsurancePolicyNumber string            `json:"insurancePolicyNumber"`
	InsuranceDate         string            `json:"insuranceDate"`
	InsuranceAmount       string            `json:"insuranceAmount"`
	Consignor             party             `json:"consignor"`
	Consignee             party             `json:"consignee"`
	GSTPayableBy          string            `json:"gstPayableBy"`
	VehicleNumber         string            `json:"vehicleNumber"`
	Origin                string            `json:"origin"`
	Destination           string            `json:"destination"`
	EWayBillNumber        string            `json:"eWayBillNumber"`
	Items                 []consignmentItem `json:"items"`
	Charges               charges           `json:"charges"`
	Total                 string            `json:"total"`
	DeclaredValue         string            `json:"declaredValue"`
	VehicleType           string            `json:"vehicleType"`
	ReceiverName          string            `json:"receiverName"`
	Notes                 string            `json:"notes"`
	CreatedAt             time.Time         `json:"createdAt"`
}

type apiError struct {
	Error string `json:"error"`
}

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var errInvalidConsignmentID = errors.New("invalid consignment ID")

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data/consignments"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/consignments", createConsignment(dataDir))
	mux.HandleFunc("GET /api/consignments/{id}/pdf", getConsignmentPDF(dataDir))
	mux.HandleFunc("GET /api/consignments/{id}", getConsignment(dataDir))

	address := os.Getenv("ADDR")
	if address == "" {
		address = ":8080"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           cors(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Consignment API listening on %s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func createConsignment(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var record consignment
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid form data: "+err.Error())
			return
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "Request body must contain a single JSON object.")
			return
		}
		if err := validate(record); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		record.ID = fmt.Sprintf("%s-%d", safeFilename(record.ConsignmentNumber), time.Now().UTC().UnixNano())
		record.CreatedAt = time.Now().UTC()
		if err := os.MkdirAll(dataDir, 0750); err != nil {
			log.Printf("create data directory: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not save the consignment.")
			return
		}

		path := filepath.Join(dataDir, record.ID+".json")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			log.Printf("create consignment record: %v", err)
			writeError(w, http.StatusInternalServerError, "Could not save the consignment.")
			return
		}
		encodeErr := json.NewEncoder(file).Encode(record)
		closeErr := file.Close()
		if encodeErr != nil || closeErr != nil {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				log.Printf("remove incomplete consignment record: %v", removeErr)
			}
			log.Printf("write consignment record: encode=%v close=%v", encodeErr, closeErr)
			writeError(w, http.StatusInternalServerError, "Could not save the consignment.")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": record.ID})
	}
}

func getConsignment(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		record, err := readConsignment(dataDir, r.PathValue("id"))
		if err != nil {
			writeReadError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, record)
	}
}

func getConsignmentPDF(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		record, err := readConsignment(dataDir, id)
		if err != nil {
			writeReadError(w, err)
			return
		}
		content, err := renderConsignmentPDF(record)
		if err != nil {
			log.Printf("generate PDF for consignment %q: %v", id, err)
			writeError(w, http.StatusInternalServerError, "Could not generate the consignment PDF.")
			return
		}

		filename := "consignment-" + safeFilename(record.ConsignmentNumber) + ".pdf"
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			log.Printf("write PDF for consignment %q: %v", id, err)
		}
	}
}

func readConsignment(dataDir, id string) (consignment, error) {
	if !safeID.MatchString(id) {
		return consignment{}, errInvalidConsignmentID
	}
	file, err := os.Open(filepath.Join(dataDir, id+".json"))
	if err != nil {
		return consignment{}, err
	}
	defer file.Close()

	var record consignment
	if err := json.NewDecoder(file).Decode(&record); err != nil {
		return consignment{}, fmt.Errorf("decode consignment record %q: %w", id, err)
	}
	return record, nil
}

func writeReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, errInvalidConsignmentID) {
		writeError(w, http.StatusBadRequest, "Invalid consignment ID.")
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "Consignment not found.")
		return
	}
	log.Printf("read consignment record: %v", err)
	writeError(w, http.StatusInternalServerError, "Could not read the consignment.")
}

func validate(record consignment) error {
	if strings.TrimSpace(record.Transporter.Name) == "" {
		return errors.New("Transporter name is required.")
	}
	if strings.TrimSpace(record.ConsignmentNumber) == "" {
		return errors.New("Consignment note number is required.")
	}
	if _, err := time.Parse("2006-01-02", record.Date); err != nil {
		return errors.New("A valid document date is required.")
	}
	if strings.TrimSpace(record.Consignor.Name) == "" || strings.TrimSpace(record.Consignee.Name) == "" {
		return errors.New("Consignor and consignee names are required.")
	}
	if strings.TrimSpace(record.Origin) == "" || strings.TrimSpace(record.Destination) == "" {
		return errors.New("Origin and destination are required.")
	}
	if len(record.Items) == 0 {
		return errors.New("At least one consignment item is required.")
	}
	if record.InsuranceStatus != "insured" && record.InsuranceStatus != "not_insured" {
		return errors.New("Choose whether the consignment is insured.")
	}
	return nil
}

func safeFilename(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-_")
	if result == "" {
		return "consignment"
	}
	return result
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}
