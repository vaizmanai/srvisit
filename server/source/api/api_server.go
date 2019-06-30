package api

import (
    "../common"
    "../component/client"
    "encoding/json"
    "net/http"
)

func HandleGetClientsList(w http.ResponseWriter, r *http.Request) {
    b, err := json.Marshal(client.GetAllClientsList())
    if err == nil {
        w.Write(b)
        return
    }
    common.LogAdd(common.MessError, "handleAdminTest: "+err.Error())
    http.Error(w, "couldn't service this request", http.StatusInternalServerError)
}
