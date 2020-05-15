// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
)

// NewPostNodeAddrParams creates a new PostNodeAddrParams object
// no default values defined in spec.
func NewPostNodeAddrParams() PostNodeAddrParams {

	return PostNodeAddrParams{}
}

// PostNodeAddrParams contains all the bound params for the post node addr operation
// typically these are obtained from a http.Request
//
// swagger:parameters PostNodeAddr
type PostNodeAddrParams struct {

	// HTTP Request Object
	HTTPRequest *http.Request `json:"-"`

	/*libp2p style addr of remote node
	  Required: true
	  In: path
	*/
	Addr string
}

// BindRequest both binds and validates a request, it assumes that complex things implement a Validatable(strfmt.Registry) error interface
// for simple values it will use straight method calls.
//
// To ensure default values, the struct must have been initialized with NewPostNodeAddrParams() beforehand.
func (o *PostNodeAddrParams) BindRequest(r *http.Request, route *middleware.MatchedRoute) error {
	var res []error

	o.HTTPRequest = r

	rAddr, rhkAddr, _ := route.Params.GetOK("addr")
	if err := o.bindAddr(rAddr, rhkAddr, route.Formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

// bindAddr binds and validates parameter Addr from path.
func (o *PostNodeAddrParams) bindAddr(rawData []string, hasKey bool, formats strfmt.Registry) error {
	var raw string
	if len(rawData) > 0 {
		raw = rawData[len(rawData)-1]
	}

	// Required: true
	// Parameter is provided by construction from the route

	o.Addr = raw

	return nil
}