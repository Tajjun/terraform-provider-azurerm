package azure

import (
	"fmt"

	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
)

//Frontdoor name must begin with a letter or number, end with a letter or number and may contain only letters, numbers or hyphens.
func ValidateFrontDoorName(i interface{}, k string) (_ []string, errors []error) {
	if m, regexErrs := validate.RegExHelper(i, k, `(^[\da-zA-Z])([-\da-zA-Z]{3,61})([\da-zA-Z]$)`); !m {
		errors = append(regexErrs, fmt.Errorf(`%q must be between 5 and 63 characters in length and begin with a letter or number, end with a letter or number and may contain only letters, numbers or hyphens.`, k))
	}

	return nil, errors
}

func ValidateBackendPoolRoutingRuleName(i interface{}, k string) (_ []string, errors []error) {
	if m, regexErrs := validate.RegExHelper(i, k, `(^[\da-zA-Z])([-\da-zA-Z]{1,88})([\da-zA-Z]$)`); !m {
		errors = append(regexErrs, fmt.Errorf(`%q must be between 1 and 90 characters in length and begin with a letter or number, end with a letter or number and may contain only letters, numbers or hyphens.`, k))
	}

	return nil, errors
}