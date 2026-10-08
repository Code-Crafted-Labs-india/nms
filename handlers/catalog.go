package handlers

import (
	"encoding/json"
	"net/http"

	"nms-middleware/db"
)

type DeviceModel struct {
	Series    string `json:"series"`
	ModelName string `json:"modelName"`
}

// DeviceModelsHandler exposes the supported hardware catalog used by the
// device onboarding form. It intentionally returns catalog metadata only.
func DeviceModelsHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}

		rows, err := database.Pool.Query(r.Context(), `
			SELECT series, model_name FROM device_models
			WHERE model_name <> 'unknown_discovered'
			ORDER BY series, model_name
		`)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "device catalog unavailable"})
			return
		}
		defer rows.Close()

		models := make([]DeviceModel, 0)
		for rows.Next() {
			var model DeviceModel
			if err := rows.Scan(&model.Series, &model.ModelName); err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "device catalog unavailable"})
				return
			}
			models = append(models, model)
		}
		if err := rows.Err(); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "device catalog unavailable"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models)
	}
}
