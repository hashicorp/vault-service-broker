package credhub_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	credhub "github.com/cloudfoundry-community/go-credhub"
	"github.com/gorilla/mux"
	uuid "github.com/nu7hatch/gouuid"
)

type credentialFile map[string][]credhub.Credential

func authHandler(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.ToLower(authHeader) != "bearer abcd" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		http.HandlerFunc(next).ServeHTTP(w, r)
	})
}

// MockCredhubServer will create a mock server that is useful for unit testing
func mockCredhubServer() *httptest.Server {
	router := mux.NewRouter()

	router.HandleFunc("/info", infoHandler).Methods(http.MethodGet)
	router.Handle("/api/v1/data", authHandler(getCredentials)).Methods(http.MethodGet)
	router.Handle("/api/v1/data/1234", authHandler(getCredentialsByID)).Methods(http.MethodGet)
	router.Handle("/api/v1/permissions", authHandler(getPermissions)).Methods(http.MethodGet)
	router.Handle("/some-url", authHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello world"))
		return
	})).Methods(http.MethodGet)

	router.Handle("/api/v1/data", authHandler(postCredentials)).Methods(http.MethodPost)
	router.Handle("/api/v1/data/regenerate", authHandler(regenerateCredentials)).Methods(http.MethodPost)
	router.Handle("/api/v1/permissions", authHandler(postPermissions)).Methods(http.MethodPost)

	router.Handle("/api/v1/data", authHandler(putCredentials)).Methods(http.MethodPut)

	router.Handle("/api/v1/data", authHandler(deleteCredentials)).Methods(http.MethodDelete)
	router.Handle("/api/v1/permissions", authHandler(deletePermissions)).Methods(http.MethodDelete)

	router.PathPrefix("/badjson").Handler(authHandler(badJSON))

	return httptest.NewTLSServer(router)
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	body := make(map[string]interface{})

	url := make(map[string]string)
	url["url"] = mockUaaServer().URL

	body["auth-server"] = url

	var out []byte
	var err error
	if out, err = json.Marshal(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(out)
}

func getCredentials(w http.ResponseWriter, r *http.Request) {
	key := "data"

	var creds []credhub.Credential
	var err error

	pathParam := r.FormValue("path")
	name := r.FormValue("name")
	paths := r.FormValue("paths")
	nameLike := r.FormValue("name-like")

	switch {
	case pathParam != "":
		key = "credentials"
		creds, err = returnCredentialsFromFile("bypath", pathParam, key, w, r)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	case name != "":
		creds, err = returnCredentialsFromFile("byname", name, key, w, r)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	case paths == "true":
		directWriteFile("testdata/credentials/allpaths.json", w, r)
		return
	case nameLike != "":
		key = "credentials"
		creds, err = returnCredentialsFromFile("bypath", "/concourse/common", key, w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		idxs := make([]int, 0, len(creds))
		for idx, cred := range creds {
			if !strings.Contains(strings.ToLower(cred.Name), strings.ToLower(nameLike)) {
				// get the list of bad indices in high to low order so as to most easily delete them
				idxs = append([]int{idx}, idxs...)
			}
		}

		for _, i := range idxs {
			creds = append(creds[:i], creds[i+1:]...)
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ret := map[string]interface{}{}
	ret[key] = creds

	b, err := json.Marshal(ret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
	return
}

func getCredentialsByID(w http.ResponseWriter, r *http.Request) {
	directWriteFile("testdata/credentials/byid/1234.json", w, r)
	return
}

func getPermissions(w http.ResponseWriter, r *http.Request) {
	var err error

	name := r.FormValue("credential_name")

	if name == "/add-permission-credential" {
		fileName := "testdata/permissions/add-permissions/cred.json"
		if _, err = os.Stat(fileName); os.IsNotExist(err) {
			err = copyFile("testdata/permissions/add-permissions/base.json", fileName)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		name = "/add-permissions/cred"
	}

	directWriteFile(path.Join("testdata/permissions", name+".json"), w, r)
	return
}

func postCredentials(w http.ResponseWriter, r *http.Request) {
	var generateBody struct {
		Name   string                 `json:"name"`
		Type   credhub.CredentialType `json:"type"`
		Params map[string]interface{} `json:"parameters"`
	}

	var cred credhub.Credential
	buf, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(buf, &cred); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	if cred.Value == nil {
		if err := json.Unmarshal(buf, &generateBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if generateBody.Params != nil {
			cred.Name = generateBody.Name
			cred.Type = generateBody.Type
			cred.Value = "1234567890asdfghjkl;ZXCVBNM<$P"
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	t := time.Now()
	cred.Created = t.Format(time.RFC3339)
	buf, e := json.Marshal(cred)
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(buf)
}

func regenerateCredentials(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}

	var cred credhub.Credential
	buf, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(buf, &body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cred.Name = body.Name
	cred.Type = credhub.Password
	cred.Value = "P$<MNBVCXZ;lkjhgfdsa0987654321"
	cred.Created = time.Now().Format(time.RFC3339)
	buf, e := json.Marshal(cred)
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(buf)
}

func postPermissions(w http.ResponseWriter, r *http.Request) {
	type permbody struct {
		Name        string               `json:"credential_name"`
		Permissions []credhub.Permission `json:"permissions"`
	}

	var body permbody

	buf, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(buf, &body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if body.Name == "/add-permission-credential" {
		fp, err := os.OpenFile("testdata/permissions/add-permissions/cred.json", os.O_RDWR, 0644)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer fp.Close()

		var buf []byte
		buf, err = ioutil.ReadAll(fp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var existing permbody
		if err = json.Unmarshal(buf, &existing); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		existing.Permissions = append(existing.Permissions, body.Permissions...)
		outbuf, err := json.Marshal(existing)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fp.WriteAt(outbuf, 0)
		w.Write(outbuf)
		return
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func putCredentials(w http.ResponseWriter, r *http.Request) {
	var cred credhub.Credential
	var req struct {
		credhub.Credential
		Mode                  credhub.OverwriteMode `json:"mode"`
		AdditionalPermissions []credhub.Permission  `json:"additional_permissions,omitempty"`
	}
	buf, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(buf, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	cred.Name = req.Name
	cred.Type = req.Type
	cred.Value = req.Value

	switch req.Mode {
	case credhub.Overwrite:
		guid, err := uuid.NewV4()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		cred.ID = guid.String()
	case credhub.NoOverwrite:
		cred.ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		cred.Value = credhub.UserValueType{
			Username:     "me",
			Password:     "old",
			PasswordHash: "old-hash",
		}
	case credhub.Converge:
		v, err := credhub.UserValue(cred)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if v.Password == "super-secret" {
			cred.ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		} else {
			guid, err := uuid.NewV4()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			cred.ID = guid.String()
		}
	}

	t := time.Now()
	cred.Created = t.Format(time.RFC3339)
	buf, e := json.Marshal(cred)
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(buf)
}

func deleteCredentials(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "/some-cred" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	return
}

func deletePermissions(w http.ResponseWriter, r *http.Request) {
	var err error

	name := r.URL.Query().Get("credential_name")
	actor := r.URL.Query().Get("actor")

	if name == "/add-permission-credential" {
		fileName := "testdata/permissions/add-permissions/cred.json"
		if _, err = os.Stat(fileName); os.IsNotExist(err) {
			err = copyFile("testdata/permissions/add-permissions/base.json", fileName)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fp, err := os.OpenFile("testdata/permissions/add-permissions/cred.json", os.O_RDWR, 0644)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer fp.Close()

		buf, err := ioutil.ReadAll(fp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		retBody := struct {
			Name        string               `json:"credential_name"`
			Permissions []credhub.Permission `json:"permissions"`
		}{}

		if err = json.Unmarshal(buf, &retBody); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		newPerms := make([]credhub.Permission, 0, len(retBody.Permissions))
		for i := range retBody.Permissions {
			if strings.TrimSpace(retBody.Permissions[i].Actor) != strings.TrimSpace(actor) {
				newPerms = append(newPerms, retBody.Permissions[i])
			}
		}

		retBody.Permissions = newPerms

		output, err := json.Marshal(retBody)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fp.Seek(0, 0)
		fp.Truncate(int64(len(output)))
		fp.WriteAt(output, 0)

		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func badJSON(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{invalid}`))
}

func returnPermissionsFromFile(credentialName string) ([]credhub.Permission, error) {
	filePath := path.Join("testdata/permissions", credentialName+".json")
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	retBody := struct {
		CN          string               `json:"credential_name"`
		Permissions []credhub.Permission `json:"permissions"`
	}{}

	if err = json.Unmarshal(buf, &retBody); err != nil {
		return nil, err
	}

	return retBody.Permissions, nil
}

func returnCredentialsFromFile(query string, value string, key string, w http.ResponseWriter, r *http.Request) ([]credhub.Credential, error) {
	filePath := path.Join("testdata/credentials", query, value+".json")
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var creds []credhub.Credential

	params := r.URL.Query()
	name := params.Get("name")

	ret := make(credentialFile)

	if err = json.Unmarshal(buf, &ret); err != nil {
		return nil, err
	}

	creds = ret[key]

	if name != "" {
		currentStr := params.Get("current")
		versionsStr := params.Get("versions")

		sort.Slice(ret[key], func(i, j int) bool {
			less := strings.Compare(ret[key][i].Created, ret[key][j].Created)
			return less > 0
		})

		current, _ := strconv.ParseBool(currentStr)
		if current {
			data := ret[key][0:1]
			ret[key] = data
		} else {
			nv, _ := strconv.Atoi(versionsStr)
			if nv > 0 {
				data := ret[key][0:nv]
				ret[key] = data
			}
		}

		creds = ret[key]
	}

	return creds, nil
}

func directWriteFile(path string, w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(buf)
}

func copyFile(src, dst string) error {
	var in, out *os.File
	var err error
	if in, err = os.Open(src); err != nil {
		return err
	}
	defer in.Close()

	if out, err = os.Create(dst); err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}
