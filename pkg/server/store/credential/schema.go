package credential

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	raw "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	serviceAccountFile string
	linksHandler       = map[string]http.Handler{
		"zone":         http.HandlerFunc(zoneHandler),
		"region":       http.HandlerFunc(regionHandler),
		"machineType":  http.HandlerFunc(machineTypeHandler),
		"machineImage": http.HandlerFunc(machineImageHandler),
		"diskType":     http.HandlerFunc(diskTypeHandler),
		"network":      http.HandlerFunc(networkHandler),
	}
)

func newClient() (*raw.Service, error) {
	if serviceAccountFile == "" {
		return nil, errors.New("service account file is empty")
	}
	credJSON, err := os.ReadFile(serviceAccountFile)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	ts, err := google.CredentialsFromJSON(ctx, credJSON, raw.ComputeScope)
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, ts.TokenSource)
	return raw.NewService(ctx, option.WithHTTPClient(client))
}

func Formatter(request *types.APIRequest, resource *types.RawResource) {
	if strings.EqualFold(resource.APIObject.Data().String("provider"), "google") {
		for key := range linksHandler {
			resource.Links[key] = request.URLBuilder.Link(resource.Schema, resource.ID, key)
		}
		resource.Schema.LinkHandlers = linksHandler
		serviceAccountFile = resource.APIObject.Data().Map("secrets").String("service-account-file")
	} else {
		for key := range linksHandler {
			delete(resource.Links, key)
		}
		resource.Schema.LinkHandlers = map[string]http.Handler{}
		serviceAccountFile = ""
	}
}

func zoneHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	zoneList, err := client.Zones.List(project).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call zone list google api"))
		return
	}

	result := types.APIObjectList{}
	if zoneList != nil {
		for _, zone := range zoneList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "zone",
				ID:     strconv.FormatUint(zone.Id, 10),
				Object: zone,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func regionHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	regionList, err := client.Regions.List(project).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call region list google api"))
		return
	}

	result := types.APIObjectList{}
	if regionList != nil {
		for _, region := range regionList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "region",
				ID:     strconv.FormatUint(region.Id, 10),
				Object: region,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func machineTypeHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	zone := apiContext.Query.Get("zone")
	if zone == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: zone"))
		return
	}
	machineTypeList, err := client.MachineTypes.List(project, zone).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call machineType list google api"))
		return
	}

	result := types.APIObjectList{}
	if machineTypeList != nil {
		for _, machineType := range machineTypeList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "machineType",
				ID:     strconv.FormatUint(machineType.Id, 10),
				Object: machineType,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func machineImageHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	machineImageList, err := client.MachineImages.List(project).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call machineImage list google api"))
		return
	}

	result := types.APIObjectList{}
	if machineImageList != nil {
		for _, machineImage := range machineImageList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "machineImage",
				ID:     strconv.FormatUint(machineImage.Id, 10),
				Object: machineImage,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func diskTypeHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	zone := apiContext.Query.Get("zone")
	if zone == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: zone"))
		return
	}
	diskTypeList, err := client.DiskTypes.List(project, zone).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call diskType list google api"))
		return
	}

	result := types.APIObjectList{}
	if diskTypeList != nil {
		for _, diskType := range diskTypeList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "diskType",
				ID:     strconv.FormatUint(diskType.Id, 10),
				Object: diskType,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func networkHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	client, err := newClient()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to initialize google client"))
		return
	}

	project := apiContext.Query.Get("project")
	if project == "" {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.MissingRequired, "missing required query values: project"))
		return
	}
	networkList, err := client.Networks.List(project).Do()
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to call network list google api"))
		return
	}

	result := types.APIObjectList{}
	if networkList != nil {
		for _, network := range networkList.Items {
			result.Objects = append(result.Objects, types.APIObject{
				Type:   "network",
				ID:     strconv.FormatUint(network.Id, 10),
				Object: network,
			})
		}
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}
