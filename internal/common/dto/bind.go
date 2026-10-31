package dto

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func BindJSON(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		ErrorResponse(c, 400, "Invalid JSON request")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		ErrorResponse(c, 400, "Expected one JSON object")
		return false
	}
	if err := binding.Validator.ValidateStruct(value); err != nil {
		ErrorResponse(c, 400, "Invalid request fields")
		return false
	}
	return true
}
