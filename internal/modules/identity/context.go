package identity

import "platform-service/internal/models"

func (s *Service) ReissueForContext(userID, orgID, orgRole string) (*AuthResult, error) {
	user, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.FindOrganizationByID(orgID)
	if err != nil {
		return nil, err
	}
	user.OrgID = orgID
	user.CurrentOrgID = orgID
	user.OrgRole = orgRole
	token, err := s.generateToken(*user, *org, orgRole)
	if err != nil {
		return nil, err
	}
	profile, err := s.buildProfile(s.repo.DB(), *user, orgID)
	if err != nil {
		return nil, err
	}
	return &AuthResult{AccessToken: token, User: *profile}, nil
}

func (s *Service) BuildProfileForUser(user models.User, orgID string) (*UserProfile, error) {
	return s.buildProfile(s.repo.DB(), user, orgID)
}
