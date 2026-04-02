package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type listActiveAgreementsResponse struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	PublishedOn string `json:"published_on"`
}

// @Summary List active agreements
// @Description get the list of all active agreements
// @Tags agreements
// @Accept json
// @Produce json
// @Success 200 {array} listActiveAgreementsResponse
// @Router /v1/agreements [get]
func (server *Server) listAgreements(ctx *gin.Context) {
	agreements, err := server.store.ListActiveAgreements(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := make([]listActiveAgreementsResponse, len(agreements))
	for i, a := range agreements {
		response[i] = listActiveAgreementsResponse{
			Type:        a.Type,
			Title:       a.Title,
			Version:     a.Version,
			PublishedOn: a.PublishedOn.Time.Format("2006-01-02"),
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// @Summary Get active agreement by type
// @Description get the content of an active agreement by its type
// @Tags agreements
// @Accept json
// @Produce json
// @Param type path string true "Agreement Type"
// @Success 200 {object} db.Agreement
// @Router /v1/agreements/{type} [get]
func (server *Server) getAgreement(ctx *gin.Context) {
	agreementType := ctx.Param("type")

	agreement, err := server.store.GetActiveAgreementByType(ctx, agreementType)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, agreement)
}
