package grpcserver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	companyv1 "gitlab.com/shutovAndrey/xm/gen/company/v1"
	"gitlab.com/shutovAndrey/xm/internal/company"
)

func TestTypeFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		in       companyv1.CompanyType
		want     company.Type
		wantCode codes.Code
	}{
		{name: "corporations", in: companyv1.CompanyType_COMPANY_TYPE_CORPORATIONS, want: "Corporations"},
		{name: "non profit", in: companyv1.CompanyType_COMPANY_TYPE_NON_PROFIT, want: "NonProfit"},
		{name: "cooperative", in: companyv1.CompanyType_COMPANY_TYPE_COOPERATIVE, want: "Cooperative"},
		{name: "sole proprietorship", in: companyv1.CompanyType_COMPANY_TYPE_SOLE_PROPRIETORSHIP, want: "Sole Proprietorship"},
		{name: "unspecified", in: companyv1.CompanyType_COMPANY_TYPE_UNSPECIFIED, wantCode: codes.InvalidArgument},
		{name: "unknown enum value", in: companyv1.CompanyType(99), wantCode: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typeFromRequest(tt.in)
			if tt.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTypeToProto(t *testing.T) {
	tests := []struct {
		name string
		in   company.Type
		want companyv1.CompanyType
	}{
		{name: "corporations", in: "Corporations", want: companyv1.CompanyType_COMPANY_TYPE_CORPORATIONS},
		{name: "non profit", in: "NonProfit", want: companyv1.CompanyType_COMPANY_TYPE_NON_PROFIT},
		{name: "cooperative", in: "Cooperative", want: companyv1.CompanyType_COMPANY_TYPE_COOPERATIVE},
		{name: "sole proprietorship", in: "Sole Proprietorship", want: companyv1.CompanyType_COMPANY_TYPE_SOLE_PROPRIETORSHIP},
		{name: "unknown db value", in: "Partnership", want: companyv1.CompanyType_COMPANY_TYPE_UNSPECIFIED},
		{name: "empty", in: "", want: companyv1.CompanyType_COMPANY_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, typeToProto(tt.in))
		})
	}
}

func TestTypeMappingRoundTrip(t *testing.T) {
	for proto := range types {
		dbType, err := typeFromRequest(proto)
		require.NoError(t, err)
		assert.Equal(t, proto, typeToProto(dbType), "proto -> db -> proto must round-trip for %s", proto)
	}
}

func TestToStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{name: "not found", err: company.ErrNotFound, wantCode: codes.NotFound, wantMsg: company.ErrNotFound.Error()},
		{name: "name taken", err: company.ErrNameTaken, wantCode: codes.AlreadyExists, wantMsg: company.ErrNameTaken.Error()},
		{name: "invalid id", err: company.ErrInvalidID, wantCode: codes.InvalidArgument, wantMsg: company.ErrInvalidID.Error()},
		{name: "wrapped domain error", err: fmt.Errorf("repo: %w", company.ErrNotFound), wantCode: codes.NotFound},
		{name: "unknown error hides details", err: errors.New("connection reset by peer"), wantCode: codes.Internal, wantMsg: "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, ok := status.FromError(toStatus(tt.err))
			require.True(t, ok, "must be a gRPC status")
			assert.Equal(t, tt.wantCode, st.Code())
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, st.Message())
			}
		})
	}
}
