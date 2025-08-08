package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	acme "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	//"github.com/nrdcg/porkbun"
	pbclient "github.com/hraedon/cert-manager-porkbun-webhook/internal/pbclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var zapLogger, _ = zap.NewProduction()

type PorkbunSolver struct {
	kube *kubernetes.Clientset
}

func (e *PorkbunSolver) Name() string {
	return "porkbun"
}

type PorkbunDNSProviderConfig struct {
	SecretNameRef      string `json:"secretNameRef"`
	ApiKeySecretRef    string `json:"apiKeySecretRef"`
	SecretKeySecretRef string `json:"secretKeySecretRef"`
}

type Config struct {
	Client *pbclient.Client
}

func (e *PorkbunSolver) Present(ch *acme.ChallengeRequest) error {
    slogger := zapLogger.Sugar()
    slogger.Infof("Handling present request for %q %q", ch.ResolvedFQDN, ch.Key)

    cfg, err := clientConfig(e, ch)
    if err != nil { return errors.Wrap(err, "initialization error") }

    client := cfg.Client
    ctx := context.Background()

    zone := strings.TrimSuffix(ch.ResolvedZone, ".")   // e.g. "hraedon.com"
    fqdn := strings.TrimSuffix(ch.ResolvedFQDN, ".")   // e.g. "_acme-challenge.portainer.hraedon.com"

    records, err := client.RetrieveRecords(ctx, zone)
    if err != nil { return errors.Wrap(err, "retrieve records error") }

    for _, r := range records {
        if r.Type == "TXT" && r.Name == fqdn && r.Content == ch.Key {
            slogger.Infof("Record %s already present", r.ID)
            return nil
        }
    }

    // Create TXT record with full name (matches what you tested by hand)
    id, err := client.CreateTXT(ctx, zone, fqdn, ch.Key, "60")
    if err != nil { return errors.Wrap(err, "create record error") }

    slogger.Infof("Created record %s", id)
    return nil
}

func (e *PorkbunSolver) CleanUp(ch *acme.ChallengeRequest) error {
    slogger := zapLogger.Sugar()
    slogger.Infof("Handling cleanup request for %q %q", ch.ResolvedFQDN, ch.Key)

    cfg, err := clientConfig(e, ch)
    if err != nil { return errors.Wrap(err, "initialization error") }

    client := cfg.Client
    ctx := context.Background()

    zone := strings.TrimSuffix(ch.ResolvedZone, ".")
    fqdn := strings.TrimSuffix(ch.ResolvedFQDN, ".")

    records, err := client.RetrieveRecords(ctx, zone)
    if err != nil { return errors.Wrap(err, "retrieve records error") }

    for _, r := range records {
        if r.Type == "TXT" && r.Name == fqdn && r.Content == ch.Key {
            if err := client.DeleteRecord(ctx, zone, r.ID); err != nil {
                return errors.Wrap(err, "delete record error")
            }
            slogger.Infof("Deleted record %s", r.ID)
            return nil
        }
    }

    slogger.Info("No matching record to delete")
    return nil
}

func (e *PorkbunSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	slogger := zapLogger.Sugar()
	slogger.Info("Initializing")

	kube, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return errors.Wrap(err, "kube client creation error")
	}

	e.kube = kube
	return nil
}

func New() webhook.Solver {
	return &PorkbunSolver{}
}

// Config ------------------------------------------------------
func stringFromSecretData(secretData *map[string][]byte, key string) (string, error) {
	data, ok := (*secretData)[key]
	if !ok {
		return "", errors.New(fmt.Sprintf("key %q not found in secret data", key))
	}

	return string(data), nil
}

func loadConfig(cfgJSON *extapi.JSON) (PorkbunDNSProviderConfig, error) {
	cfg := PorkbunDNSProviderConfig{}

	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, errors.Wrap(err, fmt.Sprintf("error decoding solver config: %v", err))
	}
	return cfg, nil
}

func clientConfig(c *PorkbunSolver, ch *acme.ChallengeRequest) (Config, error) {
	var config Config

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return config, err
	}

	secretName := cfg.SecretNameRef
	apiKeyRef := cfg.ApiKeySecretRef
	secretKeyRef := cfg.SecretKeySecretRef

	sec, err := c.kube.CoreV1().Secrets(ch.ResourceNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return config, errors.Wrap(err, fmt.Sprintf("unable to get secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err))
	}

	apiKey, err := stringFromSecretData(&sec.Data, apiKeyRef)
	if err != nil {
		return config, errors.Wrap(err, fmt.Sprintf("unable to get api-key from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err))
	}

	secretKey, err := stringFromSecretData(&sec.Data, secretKeyRef)
	if err != nil {
		return config, errors.Wrap(err, fmt.Sprintf("unable to get secret-key from secret `%s/%s`; %v", secretName, ch.ResourceNamespace, err))
	}

	config.Client = pbclient.New(apiKey, secretKey)
	return config, nil
}
