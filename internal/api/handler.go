package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/script3/soroban-governor-backend/internal/db"
	"github.com/script3/soroban-governor-backend/internal/governor"
)

type Handler struct {
	store  *db.Store
	router *http.ServeMux
}

func NewHandler(store *db.Store) *Handler {
	h := &Handler{
		store:  store,
		router: http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")

	h.router.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.router.HandleFunc("GET /health", h.handleHealth)
	h.router.HandleFunc("GET /{contractId}/proposals/{proposalId}", h.handleGetProposal)

	h.router.HandleFunc("GET /{contractId}/proposals", h.handleGetProposals)
	h.router.HandleFunc("GET /{contractId}/proposals/{proposalId}/votes", h.handleGetVotes)
	h.router.HandleFunc("GET /{contractId}/events", h.handleGetEvents)
}

// handleHealth returns service health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	curUnix := time.Now().Unix()

	lastLedger, lastClostTime, err := h.store.GetStatus(r.Context(), "indexer")
	if err != nil {
		slog.Error("Failed to get last indexed ledger", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get health status")
		return
	}

	// If the indexer has not processed any ledgers in the last 2 minutes, consider unhealthy
	if lastClostTime == 0 || curUnix-lastClostTime > 120 {
		slog.Warn("Indexer is behind", "last_indexed_ledger", lastLedger, "last_close_time", lastClostTime, "time_since_close", curUnix-lastClostTime)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("too long since last indexed ledger %d, closed %ds ago", lastLedger, curUnix-lastClostTime))
		return
	}
	respondJSON(w, http.StatusOK, map[string]uint32{"status": lastLedger})
}

// handleGetProposal retrieves a single proposal by contract ID and proposal ID
func (h *Handler) handleGetProposal(w http.ResponseWriter, r *http.Request) {
	contractId := r.PathValue("contractId")
	proposalIdStr := r.PathValue("proposalId")

	proposalId, err := strconv.ParseUint(proposalIdStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid proposal_id")
		return
	}

	proposalKey := governor.EncodeProposalKey(contractId, uint32(proposalId))
	proposal, err := h.store.GetProposal(r.Context(), proposalKey)
	if err != nil {
		slog.Error("Failed to get proposal", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retrieve proposal")
		return
	}

	if proposal == nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}

	respondJSON(w, http.StatusOK, proposal)
}

// handleGetProposals retrieves all proposals for a contract with pagination
func (h *Handler) handleGetProposals(w http.ResponseWriter, r *http.Request) {
	contractId := r.PathValue("contractId")

	proposals, err := h.store.GetProposalsByContractId(
		r.Context(),
		contractId,
	)
	if err != nil {
		slog.Error("Failed to get proposals", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retrieve proposals")
		return
	}

	// Build response with pagination metadata
	respondJSON(w, http.StatusOK, proposals)
}

// handleGetVotes retrieves all votes for a specific proposal with pagination
func (h *Handler) handleGetVotes(w http.ResponseWriter, r *http.Request) {
	contractId := r.PathValue("contractId")
	proposalIdStr := r.PathValue("proposalId")

	proposalId, err := strconv.ParseUint(proposalIdStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid proposal_id")
		return
	}

	votes, err := h.store.GetVotesByProposal(
		r.Context(),
		contractId,
		uint32(proposalId),
	)
	if err != nil {
		slog.Error("Failed to get votes", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retrieve votes")
		return
	}

	respondJSON(w, http.StatusOK, votes)
}

// handleGetEvents retrieves all events for a contract with pagination
func (h *Handler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	contractId := r.PathValue("contractId")

	events, err := h.store.GetEventsByContractId(
		r.Context(),
		contractId,
	)
	if err != nil {
		slog.Error("Failed to get events", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to retrieve events")
		return
	}

	respondJSON(w, http.StatusOK, events)
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}
