package credhub

import "encoding/json"

// InterpolateCredentials will take a string representation of a VCAP_SERVICES
// json variable, and interpolate any services whose credentials block consists
// only of credhub-ref. It will return the interpolated JSON as a string
func (c *Client) InterpolateCredentials(vcapServices string) (string, error) {
	var err error

	type vcapService map[string]interface{}

	services := make(map[string][]vcapService)
	if err = json.Unmarshal([]byte(vcapServices), &services); err != nil {
		return "", err
	}

	for serviceType := range services {
		for i := range services[serviceType] {
			credRefIntf := services[serviceType][i]["credentials"]
			credRef, ok := credRefIntf.(map[string]interface{})
			if ok && len(credRef) == 1 {
				ref, ok := credRef["credhub-ref"]
				if ok {
					var resolvedCreds []Credential
					credName := ref.(string)
					resolvedCreds, err = c.getByName(credName, true, 1)
					if err != nil {
						return "", err
					}

					services[serviceType][i]["credentials"] = resolvedCreds[0].Value
				}
			}
		}
	}

	// can't really encounter an error here, since everything has come from
	// previously unmarshalled json, so it should marshal just fine
	output, _ := json.Marshal(services)
	return string(output), nil
}
