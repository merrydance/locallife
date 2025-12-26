package val

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateIDCard(t *testing.T) {
	testCases := []struct {
		name     string
		idCard   string
		wantErr  bool
	}{
		{
			name:    "Valid ID Card",
			idCard:  "110101199001011234", // 格式正确
			wantErr: false,
		},
		{
			name:    "Valid ID Card with X",
			idCard:  "11010119900101123X", // 校验码为X
			wantErr: false,
		},
		{
			name:    "Valid ID Card with lowercase x",
			idCard:  "11010119900101123x", // 小写x也应该接受
			wantErr: false,
		},
		{
			name:    "Invalid length - too short",
			idCard:  "1101011990010112",
			wantErr: true,
		},
		{
			name:    "Invalid length - too long",
			idCard:  "1101011990010112345",
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			idCard:  "11010119900101123A", // A不是有效的校验码字符
			wantErr: true,
		},
		{
			name:    "Empty string",
			idCard:  "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIDCard(tc.idCard)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePhone(t *testing.T) {
	testCases := []struct {
		name    string
		phone   string
		wantErr bool
	}{
		{
			name:    "Valid phone 138",
			phone:   "13812345678",
			wantErr: false,
		},
		{
			name:    "Valid phone 199",
			phone:   "19912345678",
			wantErr: false,
		},
		{
			name:    "Invalid - too short",
			phone:   "1381234567",
			wantErr: true,
		},
		{
			name:    "Invalid - starts with 12",
			phone:   "12812345678",
			wantErr: true,
		},
		{
			name:    "Invalid - contains letters",
			phone:   "1381234567a",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePhone(tc.phone)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
