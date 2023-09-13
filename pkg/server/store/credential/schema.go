package credential

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	raw "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

func newClient(id string) (*raw.Service, error) {
	credID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}

	credential, err := common.DefaultDB.GetCredential(credID)
	if err != nil {
		return nil, err
	}

	secrets := map[string]string{}
	if err = json.Unmarshal(credential.Secrets, &secrets); err != nil {
		return nil, err
	}

	var f string
	f, ok := secrets["service-account-file"]
	if !ok || f == "" {
		return nil, errors.New("service account file is empty")
	}

	credJSON, err := os.ReadFile(f)
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
	resource.Links["forms"] = request.URLBuilder.Link(resource.Schema, resource.ID, "forms")
	resource.Schema.LinkHandlers = map[string]http.Handler{"forms": http.HandlerFunc(formsHandler)}
}

func formsHandler(_ http.ResponseWriter, r *http.Request) {
	result := types.APIObjectList{}
	apiContext := types.GetAPIContext(r.Context())

	provider := apiContext.Query.Get("provider")
	if provider != "google" {
		apiContext.WriteError(apierror.WrapAPIError(nil, validation.ServerError, "provider not support"))
		return
	}

	ss := strings.Split(strings.Split(apiContext.Request.RequestURI, "?")[0], "/")
	credential := ss[len(ss)-1]

	var err error
	var maxResults int64
	var returnPartialSuccess bool
	project := apiContext.Query.Get("project")
	zone := apiContext.Query.Get("zone")
	method := apiContext.Query.Get("method")
	maxResultsStr := apiContext.Query.Get("maxResults")
	pageToken := apiContext.Query.Get("pageToken")
	filter := apiContext.Query.Get("filter")
	orderBy := apiContext.Query.Get("orderBy")
	returnPartialSuccessStr := apiContext.Query.Get("returnPartialSuccess")

	if returnPartialSuccessStr != "" {
		returnPartialSuccess, err = strconv.ParseBool(returnPartialSuccessStr)
		if err != nil {
			apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "returnPartialSuccess", err.Error()))
			return
		}
	}

	if maxResultsStr != "" {
		maxResults, err = strconv.ParseInt(maxResultsStr, 10, 64)
		if err != nil {
			apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "maxResults", err.Error()))
			return
		}
	}

	if credential == "" {
		apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "credentail", ""))
		return
	}
	if project == "" {
		apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "project", ""))
		return
	}
	if method == "" {
		apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "method", ""))
		return
	}

	client, err := newClient(credential)
	if err != nil {
		apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}

	switch method {
	case "zone":
		listCall := client.Zones.List(project)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	case "region":
		listCall := client.Regions.List(project)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	case "network":
		listCall := client.Networks.List(project)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	case "machineType":
		if zone == "" {
			apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "zone", ""))
			return
		}

		listCall := client.MachineTypes.List(project, zone)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	case "image":
		listCall := client.Images.List(project)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	case "diskType":
		if zone == "" {
			apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "zone", ""))
			return
		}

		listCall := client.DiskTypes.List(project, zone)
		if pageToken != "" {
			listCall = listCall.PageToken(pageToken)
		}
		if filter != "" {
			listCall = listCall.Filter(filter)
		}
		if orderBy != "" {
			listCall = listCall.OrderBy(orderBy)
		}
		if returnPartialSuccess {
			listCall = listCall.ReturnPartialSuccess(returnPartialSuccess)
		}
		if maxResults > 0 {
			listCall = listCall.MaxResults(maxResults)
		}

		list, err := listCall.Do()
		if err != nil {
			apiContext.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		if list != nil {
			for _, item := range list.Items {
				result.Objects = append(result.Objects, types.APIObject{
					ID:     credential,
					Object: item,
				})
			}
			result.Continue = list.NextPageToken
		}
	default:
		apiContext.WriteError(apierror.NewAPIError(validation.ServerError, "method not support"))
		return
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}
