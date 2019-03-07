package metadata

import (
	"fmt"
	"github.com/mmmorris1975/aws-runas/lib/config"
	"github.com/mmmorris1975/simple-logger"
	"html/template"
	"net"
	"net/http"
)

const (
	// EC2MetadataIp is the address used to contact the metadata service, per AWS
	EC2MetadataIp = "169.254.169.254"
	// EC2MetadataCredentialPath is the base path for instance profile credentials in the metadata service
	EC2MetadataCredentialPath = "/latest/meta-data/iam/security-credentials/"
	// ListRolePath is the http server path to list the configured roles
	ListRolePath = "/list-roles"
	// MfaPath is the websocket endpoint for using MFA
	MfaPath = "/mfa"

	ctxKeyProfile = "profile"
	ctxKeyCreds   = "credentials"
)

var (
	// EC2MetadataAddress is the net.IPAddr of the EC2 metadata service
	EC2MetadataAddress *net.IPAddr

	cfg config.ConfigResolver
	log = simple_logger.StdLogger
)

func init() {
	EC2MetadataAddress, _ = net.ResolveIPAddr("ip", EC2MetadataIp)
}

type ec2MetadataOutput struct {
	Code            string
	LastUpdated     string
	Type            string
	AccessKeyId     string
	SecretAccessKey string
	Token           string
	Expiration      string
}

// NewEC2MetadataService starts an HTTP server which will listen on the EC2 metadata service path for handling
// requests for instance profile credentials.  SDKs will first look up the path in EC2MetadataCredentialPath,
// which returns the name of the instance profile in use, it then appends that value to the previous request url
// and expects the response body to contain the credential data in json format.
func NewEC2MetadataService(logLevel uint) error {
	log.SetLevel(logLevel)
	cf, err := config.NewConfigResolver(nil)
	if err != nil {
		return err
	}
	cfg = cf.WithLogger(log)

	lo, err := setupInterface()
	if err != nil {
		return err
	}
	defer removeAddress(lo, EC2MetadataAddress)

	//http.HandleFunc(EC2MetadataCredentialPath, func(writer http.ResponseWriter, request *http.Request) {
	//	p := strings.Split(request.URL.Path, "/")[1:]
	//	if len(p[len(p)-1]) < 1 {
	//		ctx := context.WithValue(context.Background(), ctxKeyProfile, profile)
	//		profileListHandler(writer, request.WithContext(ctx))
	//	} else {
	//		ctx := context.WithValue(context.Background(), ctxKeyCreds, c)
	//		profileCredentialHandler(writer, request.WithContext(ctx))
	//	}
	//})
	//
	//http.HandleFunc(ListRolePath, func(writer http.ResponseWriter, request *http.Request) {
	//	listRoleHandler(writer, request)
	//})

	http.HandleFunc("/", homeHandler)
	http.HandleFunc(MfaPath, mfaHandler)
	http.HandleFunc(EC2MetadataCredentialPath, credHandler)

	hp := net.JoinHostPort(EC2MetadataAddress.String(), "80")
	log.Infof("EC2 Metadata Service ready on %s", hp)
	return http.ListenAndServe(hp, nil)
}

func setupInterface() (string, error) {
	lo, err := discoverLoopback()
	if err != nil {
		return "", err
	}
	log.Debugf("LOOPBACK INTERFACE: %s", lo)

	if err := addAddress(lo, EC2MetadataAddress); err != nil {
		if err := removeAddress(lo, EC2MetadataAddress); err != nil {
			return "", err
		}

		if err := addAddress(lo, EC2MetadataAddress); err != nil {
			return "", err
		}
	}
	return lo, err
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	d := make(map[string]interface{})
	d["url"] = fmt.Sprintf("ws://%s%s", r.Host, MfaPath)
	d["roles"] = cfg.ListProfiles(true)
	homeTemplate.Execute(w, d)
}

func mfaHandler(w http.ResponseWriter, r *http.Request) {

}

func credHandler(w http.ResponseWriter, r *http.Request) {

}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {
  var ws = new WebSocket("{{.url}}");

  ws.onclose = function(evt) {
    ws = null;
  };

  ws.onmessage = function(evt) {
    // todo
    //if (evt.data == "Enter MFA Code") {
    //  var mfa = prompt(evt.data, "");
    //  ws.send(mfa)
    //} else {
    //  print("RESPONSE: " + evt.data);
    //}
  };

  ws.onerror = function(evt) {
    // todo
    //print("ERROR: " + evt.data);
  };

  document.getElementById("send").onclick = function(evt) {
    if (!ws) {
      return false;
    }

    // todo
    //ws.send("role");
    return false;
  };
});
</script>
</head>
<body>
URL: {{.url}}
<br>
Roles: {{.roles}}
</body>
</html>
`))

//func profileListHandler(w http.ResponseWriter, r *http.Request) {
//	p := r.Context().Value(ctxKeyProfile)
//	switch v := p.(type) {
//	case string:
//		w.Header().Set("Content-Type", "text/plain")
//		w.Write([]byte(v))
//		log.Debugf("Returning profile name %s", v)
//		log.Infof("%s %d %d", r.URL.Path, http.StatusOK, len(v))
//	default:
//		http.Error(w, "invalid profile value", http.StatusBadRequest)
//	}
//}
//
//func profileCredentialHandler(w http.ResponseWriter, r *http.Request) {
//	c := r.Context().Value(ctxKeyCreds)
//	switch t := c.(type) {
//	case *credentials.Credentials:
//		v, err := t.Get()
//		if err != nil {
//			log.Errorf("AssumeRole(): %v", err)
//			http.Error(w, "Error fetching role credentials", http.StatusInternalServerError)
//			return
//		}
//
//		// 901 seconds is the absolute minimum Expiration time so that the default awscli logic won't think
//		// our credentials are expired, and send a duplicate request.
//		output := ec2MetadataOutput{
//			Code:            "Success",
//			LastUpdated:     time.Now().UTC().Format(time.RFC3339),
//			Type:            "AWS-HMAC",
//			AccessKeyId:     v.AccessKeyID,
//			SecretAccessKey: v.SecretAccessKey,
//			Token:           v.SessionToken,
//			Expiration:      time.Now().Add(901 * time.Second).UTC().Format(time.RFC3339),
//		}
//		log.Debugf("%+v", output)
//
//		j, err := json.Marshal(output)
//		if err != nil {
//			log.Errorf("json.Marshal(): %v", err)
//			http.Error(w, "Error marshalling credentials to json", http.StatusInternalServerError)
//			return
//		}
//
//		w.Header().Set("Content-Type", "text/plain")
//		w.Write(j)
//		log.Infof("%s %d %d", r.URL.Path, http.StatusOK, len(j))
//	}
//}
//
//func listRoleHandler(w http.ResponseWriter, r *http.Request) {
//	p := cfg.ListProfiles(true)
//
//	j, err := json.Marshal(p)
//	if err != nil {
//		log.Errorf("error converting role list to json: %v", err)
//		http.Error(w, "json marshaling error", http.StatusInternalServerError)
//		return
//	}
//
//	w.Header().Set("Content-Type", "application/json")
//	w.Write(j)
//	log.Infof("%s %d %d", r.URL.Path, http.StatusOK, len(j))
//}

// todo provide an endpoint to allow switching the profile that's in use
// Will require a larger rethink of how we're dealing with credentials via this service.
// Currently, the credential provider is passed in during initialization, so we're locked into a single role
// A role may be using a different source profile, which means a distinct set of session token creds needed to assume role
// Maybe allow a nil/empty profile name during init, but if provided we can use it as the initial profile/role to use
// MFA handling needs a solution if config expects it, may need something more clever then cmdline input
//func profileSwitchHandler(w http.ResponseWriter, r *http.Request) {
//
//}