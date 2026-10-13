package grpcserver

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	companyv1 "gitlab.com/shutovAndrey/xm/gen/company/v1"
	"gitlab.com/shutovAndrey/xm/internal/company"
)

type CompanyServiceServer struct {
	companyv1.UnimplementedCompanyServiceServer

	repo *company.Repository
}

func NewCompanyServiceServer(repo *company.Repository) *CompanyServiceServer {
	return &CompanyServiceServer{repo: repo}
}

func (s *CompanyServiceServer) CreateCompany(ctx context.Context, req *companyv1.CreateCompanyRequest) (*companyv1.CreateCompanyResponse, error) {
	typ, err := typeFromRequest(req.GetType())
	if err != nil {
		return nil, err
	}

	id, createdAt, err := s.repo.Create(ctx, company.Company{
		Name:           req.GetName(),
		Description:    req.Description,
		EmployeesCount: req.GetEmployeesCount(),
		IsRegistered:   req.GetIsRegistered(),
		Type:           typ,
	})
	if err != nil {
		return nil, toStatus(err)
	}

	return &companyv1.CreateCompanyResponse{
		Id:        id,
		CreatedAt: timestamppb.New(createdAt),
	}, nil
}

func (s *CompanyServiceServer) GetCompany(ctx context.Context, req *companyv1.GetCompanyRequest) (*companyv1.GetCompanyResponse, error) {
	c, err := s.repo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}

	return &companyv1.GetCompanyResponse{
		Company: &companyv1.Company{
			Id:             c.ID,
			Name:           c.Name,
			Description:    c.Description,
			EmployeesCount: c.EmployeesCount,
			IsRegistered:   c.IsRegistered,
			Type:           typeToProto(c.Type),
		},
	}, nil
}

func (s *CompanyServiceServer) UpdateCompany(ctx context.Context, req *companyv1.UpdateCompanyRequest) (*companyv1.UpdateCompanyResponse, error) {
	typ, err := typeFromRequest(req.GetType())
	if err != nil {
		return nil, err
	}

	err = s.repo.Update(ctx, req.GetId(), company.Company{
		Name:         req.GetName(),
		Description:  req.Description,
		IsRegistered: req.GetIsRegistered(),
		Type:         typ,
	}, req.GetEmployeesDelta())
	if err != nil {
		return nil, toStatus(err)
	}

	return &companyv1.UpdateCompanyResponse{Id: req.GetId()}, nil
}

func (s *CompanyServiceServer) DeleteCompany(ctx context.Context, req *companyv1.DeleteCompanyRequest) (*companyv1.DeleteCompanyResponse, error) {
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}

	return &companyv1.DeleteCompanyResponse{}, nil
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, company.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, company.ErrNameTaken):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

var types = map[companyv1.CompanyType]company.Type{
	companyv1.CompanyType_COMPANY_TYPE_CORPORATIONS:        "Corporations",
	companyv1.CompanyType_COMPANY_TYPE_NON_PROFIT:          "NonProfit",
	companyv1.CompanyType_COMPANY_TYPE_COOPERATIVE:         "Cooperative",
	companyv1.CompanyType_COMPANY_TYPE_SOLE_PROPRIETORSHIP: "Sole Proprietorship",
}

func typeFromRequest(t companyv1.CompanyType) (company.Type, error) {
	typ, ok := types[t]
	if !ok {
		return "", status.Error(codes.InvalidArgument, "type is required")
	}
	return typ, nil
}

func typeToProto(t company.Type) companyv1.CompanyType {
	for k, v := range types {
		if v == t {
			return k
		}
	}
	return companyv1.CompanyType_COMPANY_TYPE_UNSPECIFIED
}
