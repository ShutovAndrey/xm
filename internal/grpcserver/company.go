package grpcserver

import (
	"context"

	companyv1 "gitlab.com/shutovAndrey/xm/gen/company/v1"
)

type CompanyServiceServer struct {
	companyv1.CompanyServiceServer
}

func (s *CompanyServiceServer) GetCompany(ctx context.Context, req *companyv1.GetCompanyRequest) (*companyv1.GetCompanyResponse, error) {
	return nil, nil
}
