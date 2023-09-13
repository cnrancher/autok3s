package sshkey

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"
	pkgsshkey "github.com/cnrancher/autok3s/pkg/sshkey"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

type Store struct {
	empty.Store
}

func (s *Store) Create(_ *types.APIRequest, _ *types.APISchema, data types.APIObject) (types.APIObject, error) {
	rtn := common.SSHKey{}
	if err := convert.ToObj(data.Object, &rtn); err != nil {
		return types.APIObject{}, err
	}
	if exist, _ := common.DefaultDB.SSHKeyExists(rtn.Name); exist {
		return types.APIObject{}, apierror.NewAPIError(validation.Conflict, fmt.Sprintf("sshkey %s already exists", rtn.Name))
	}

	if rtn.GenerateKey {
		if rtn.Bits%256 != 0 {
			return types.APIObject{}, apierror.NewFieldAPIError(validation.InvalidBodyContent, "bits", "ssh private key bit size should be a multiple of 256")
		}
		if err := pkgsshkey.GenerateSSHKey(&rtn); err != nil {
			return types.APIObject{}, apierror.WrapAPIError(err, validation.ServerError, "failed to generate ssh key")
		}
		// remove passphrase for response
		rtn.SSHPassphrase = ""
	} else {
		if rtn.SSHKey == "" {
			return types.APIObject{}, apierror.NewAPIError(validation.MissingRequired, "ssh private key is required when not generating new ssh key pair")
		}
		if ok, err := pkgsshkey.NeedPasswordRaw([]byte(rtn.SSHKey)); err != nil {
			return types.APIObject{}, apierror.WrapAPIError(err, validation.InvalidBodyContent, "failed to parse ssh private key")
		} else if ok && rtn.SSHPassphrase == "" {
			return types.APIObject{}, apierror.NewAPIError(validation.MissingRequired, "ssh key passphrase is required when the private is encrypted")
		}
		if err := pkgsshkey.CreateSSHKey(&rtn); err != nil {
			return types.APIObject{}, apierror.WrapAPIError(err, validation.ServerError, "failed to save ssh key")
		}
	}

	return *common.GetAPIObject(rtn), nil
}

func (s *Store) List(_ *types.APIRequest, _ *types.APISchema) (types.APIObjectList, error) {
	var rtn types.APIObjectList
	sshkeys, err := common.DefaultDB.ListSSHKey(nil)
	if err != nil {
		return rtn, err
	}
	for _, key := range sshkeys {
		key.SSHKey = ""
		obj := common.GetAPIObject(*key)
		rtn.Objects = append(rtn.Objects, *obj)
	}
	return rtn, nil
}

func (s *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	rtn, err := s.ByID(apiOp, schema, id)
	if err != nil {
		return types.APIObject{}, err
	}

	return rtn, common.DefaultDB.DeleteSSHKey(id)
}

func (s *Store) ByID(_ *types.APIRequest, _ *types.APISchema, id string) (types.APIObject, error) {
	rtn, err := common.DefaultDB.ListSSHKey(&id)
	if err != nil {
		return types.APIObject{}, err
	}
	rtn[0].SSHKey = ""
	obj := common.GetAPIObject(rtn[0])
	return *obj, nil
}

func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	return common.DefaultDB.Watch(apiOp, schema), nil
}
