package grpc

import (
	"context"
	"encoding/json"

	"github.com/miru-project/miru-core/pkg/jsExtension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func (s *MiruCoreServer) GetRepos(ctx context.Context, req *proto.GetReposRequest) (*proto.GetReposResponse, error) {
	repos, err := jsExtension.LoadExtensionRepo()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(repos)
	return &proto.GetReposResponse{Data: string(data)}, nil
}

func (s *MiruCoreServer) SetRepo(ctx context.Context, req *proto.SetRepoRequest) (*proto.SetRepoResponse, error) {
	err := jsExtension.SaveExtensionRepo(req.RepoUrl, req.Name)
	if err != nil {
		return nil, err
	}
	return &proto.SetRepoResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) DeleteRepo(ctx context.Context, req *proto.DeleteRepoRequest) (*proto.DeleteRepoResponse, error) {
	err := jsExtension.RemoveExtensionRepo(req.RepoUrl)
	if err != nil {
		return nil, err
	}
	return &proto.DeleteRepoResponse{Message: "Success"}, nil
}

func (s *MiruCoreServer) FetchRepoList(ctx context.Context, req *proto.FetchRepoListRequest) (*proto.FetchRepoListResponse, error) {
	repoList, _, err := jsExtension.FetchExtensionRepo()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(repoList)
	return &proto.FetchRepoListResponse{Data: string(data)}, nil
}
