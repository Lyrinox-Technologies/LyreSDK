package wire

import (
	"bytes"
	"fmt"

	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
)

const (
	AdminActionStatus            = "status"
	AdminActionListUsers         = "list_users"
	AdminActionSetActive         = "set_active"
	AdminActionSetAdmin          = "set_admin"
	AdminActionSetRole           = "set_role"
	AdminActionSuspendUser       = "suspend_user"
	AdminActionRestoreUser       = "restore_user"
	AdminActionBanUser           = "ban_user"
	AdminActionDeleteUser        = "delete_user"
	AdminActionDisableService    = "disable_service"
	AdminActionEnableService     = "enable_service"
	AdminActionDeleteService     = "delete_service"
	AdminActionListRBAC          = "list_rbac"
	AdminActionCreateRole        = "create_role"
	AdminActionDeleteRole        = "delete_role"
	AdminActionSetPermissions    = "set_permissions"
	AdminActionGrantRole         = "grant_role"
	AdminActionRevokeRole        = "revoke_role"
	AdminActionSendPasswordReset = "send_password_reset"
	AdminActionListContracts     = "list_contracts"
	AdminActionCreateContract    = "create_contract"
	AdminActionDeprecateContract = "deprecate_contract"
	AdminActionDeleteContract    = "delete_contract"
	AdminActionDisableProvider   = "disable_provider"
	AdminActionEnableProvider    = "enable_provider"
	AdminActionDeleteProvider    = "delete_provider"
)

// AdminRequestPayload is accepted only from an authenticated Lyre administrator.
// The status action is available to any authenticated user and exposes only that
// user's administrator capability.
type AdminRequestPayload struct {
	Action string
	UserID string
	Value  bool
	Role   string
	Data   string
}

func (p *AdminRequestPayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteString(buf, p.Action)
	rdgproto.WriteString(buf, p.UserID)
	rdgproto.WriteBool(buf, p.Value)
	rdgproto.WriteString(buf, p.Role)
	rdgproto.WriteString(buf, p.Data)
	return buf.Bytes(), nil
}

func (p *AdminRequestPayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	if p.Action, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	if p.UserID, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	p.Value, err = rdgproto.ReadBool(r)
	if err != nil || r.Len() == 0 {
		return err
	}
	p.Role, err = rdgproto.ReadString(r)
	if err != nil || r.Len() == 0 {
		return err
	}
	p.Data, err = rdgproto.ReadString(r)
	return err
}

// AdminUser contains only the identity metadata needed for administration.
// Password hashes, session tokens, MFA secrets, and service credentials never
// leave the Lyre server.
type AdminUser struct {
	ID            string
	Username      string
	Email         string
	EmailVerified bool
	Active        bool
	IsAdmin       bool
	CreatedAt     int64
	Role          string
	AccountStatus string
}

type AdminResponsePayload struct {
	Success bool
	IsAdmin bool
	Message string
	Users   []AdminUser
	RBAC    string
}

func (p *AdminResponsePayload) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	rdgproto.WriteBool(buf, p.Success)
	rdgproto.WriteBool(buf, p.IsAdmin)
	rdgproto.WriteString(buf, p.Message)
	if err := rdgproto.WriteUint32(buf, uint32(len(p.Users))); err != nil {
		return nil, err
	}
	for _, user := range p.Users {
		rdgproto.WriteString(buf, user.ID)
		rdgproto.WriteString(buf, user.Username)
		rdgproto.WriteString(buf, user.Email)
		rdgproto.WriteBool(buf, user.EmailVerified)
		rdgproto.WriteBool(buf, user.Active)
		rdgproto.WriteBool(buf, user.IsAdmin)
		if err := rdgproto.WriteUint64(buf, uint64(user.CreatedAt)); err != nil {
			return nil, err
		}
		rdgproto.WriteString(buf, user.Role)
		rdgproto.WriteString(buf, user.AccountStatus)
	}
	rdgproto.WriteString(buf, p.RBAC)
	return buf.Bytes(), nil
}

func (p *AdminResponsePayload) Unmarshal(data []byte) error {
	r := bytes.NewReader(data)
	var err error
	if p.Success, err = rdgproto.ReadBool(r); err != nil {
		return err
	}
	if p.IsAdmin, err = rdgproto.ReadBool(r); err != nil {
		return err
	}
	if p.Message, err = rdgproto.ReadString(r); err != nil {
		return err
	}
	count, err := rdgproto.ReadUint32(r)
	if err != nil {
		return err
	}
	if count > 10_000 {
		return fmt.Errorf("admin user response exceeds limit")
	}
	p.Users = make([]AdminUser, 0, count)
	for range count {
		user := AdminUser{}
		if user.ID, err = rdgproto.ReadString(r); err != nil {
			return err
		}
		if user.Username, err = rdgproto.ReadString(r); err != nil {
			return err
		}
		if user.Email, err = rdgproto.ReadString(r); err != nil {
			return err
		}
		if user.EmailVerified, err = rdgproto.ReadBool(r); err != nil {
			return err
		}
		if user.Active, err = rdgproto.ReadBool(r); err != nil {
			return err
		}
		if user.IsAdmin, err = rdgproto.ReadBool(r); err != nil {
			return err
		}
		createdAt, err := rdgproto.ReadUint64(r)
		if err != nil {
			return err
		}
		user.CreatedAt = int64(createdAt)
		if r.Len() > 0 {
			if user.Role, err = rdgproto.ReadString(r); err != nil {
				return err
			}
			if user.AccountStatus, err = rdgproto.ReadString(r); err != nil {
				return err
			}
		}
		p.Users = append(p.Users, user)
	}
	if r.Len() > 0 {
		p.RBAC, err = rdgproto.ReadString(r)
	}
	return nil
}
