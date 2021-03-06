/*******************************************************************************
*
* Copyright 2017 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package hermes

import (
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/sapcc/hermes/pkg/identity"
	"github.com/sapcc/hermes/pkg/storage"
	"github.com/sapcc/hermes/pkg/util"
	"github.com/spf13/viper"
	"log"
	"strings"
)

// ListEvent contains high-level data about an event, intended as a list item
//  The JSON annotations here are for the JSON to be returned by the API
type ListEvent struct {
	Source       string `json:"source"`
	ID           string `json:"event_id"`
	Type         string `json:"event_type"`
	Time         string `json:"event_time"`
	ResourceName string `json:"resource_name,omitempty"`
	ResourceId   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	Initiator    struct {
		TypeURI     string `json:"typeURI"`
		DomainID    string `json:"domain_id,omitempty"`
		DomainName  string `json:"domain_name,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
		ProjectName string `json:"project_name,omitempty"`
		UserID      string `json:"user_id"`
		UserName    string `json:"user_name,omitempty"`
		Host        struct {
			Agent   string `json:"agent"`
			Address string `json:"address"`
		} `json:"host"`
		ID string `json:"id"`
	} `json:"initiator"`
}

// FieldOrder maps the sort Fieldname and Order
type FieldOrder struct {
	Fieldname string
	Order     string //asc or desc
}

// Filter maps to the filtering/paging/sorting allowed by the API
type Filter struct {
	Source       string
	ResourceType string
	ResourceName string
	UserName     string
	EventType    string
	Time         map[string]string
	Offset       uint
	Limit        uint
	Sort         []FieldOrder
}

// GetEvents returns a list of matching events (with filtering)
func GetEvents(filter *Filter, tenantId string, keystoneDriver identity.Identity, eventStore storage.Storage) ([]*ListEvent, int, error) {
	storageFilter, err := storageFilter(filter, keystoneDriver, eventStore)
	if err != nil {
		return nil, 0, err
	}
	util.LogDebug("hermes.GetEvents: tenant id is %s", tenantId)
	eventDetails, total, err := eventStore.GetEvents(storageFilter, tenantId)
	if err != nil {
		return nil, 0, err
	}
	events, err := eventsList(eventDetails, keystoneDriver)
	if err != nil {
		return nil, 0, err
	}
	return events, total, err
}

func storageFilter(filter *Filter, keystoneDriver identity.Identity, eventStore storage.Storage) (*storage.Filter, error) {
	// As per the documentation, the default limit is 10
	if filter.Limit == 0 {
		filter.Limit = 10
	}

	if filter.Offset+filter.Limit > eventStore.MaxLimit() {
		return nil, fmt.Errorf("offset %d plus limit %d exceeds the maximum of %d",
			filter.Offset, filter.Limit, eventStore.MaxLimit())
	}

	storagefieldorder := []storage.FieldOrder{}
	err := copier.Copy(&storagefieldorder, &filter.Sort)
	if err != nil {
		panic("Could not copy storage field order.")
	}
	storageFilter := storage.Filter{
		Source:       filter.Source,
		ResourceType: filter.ResourceType,
		EventType:    filter.EventType,
		Time:         filter.Time,
		Offset:       filter.Offset,
		Limit:        filter.Limit,
		Sort:         storagefieldorder,
	}
	// Translate hermes.Filter to storage.Filter by filling in IDs for names
	if filter.ResourceName != "" {
		// TODO: make sure there is a resource type, then look up the corresponding name
		//storageFilter.ResourceId = resourceId
	}
	if filter.UserName != "" {
		util.LogDebug("Filtering on UserName: %s", filter.UserName)
		//userId, err := keystoneDriver.UserId(filter.UserName)
		//if err != nil {
		//	util.LogError("Could not find user ID &s for name %s", userId, filter.UserName)
		//}
		storageFilter.UserId = filter.UserName
	}
	return &storageFilter, nil
}

// Construct ListEvents - Optionally (default off) add the names for IDs in the events
func eventsList(eventDetails []*storage.EventDetail, keystoneDriver identity.Identity) ([]*ListEvent, error) {
	var events []*ListEvent
	for _, storageEvent := range eventDetails {
		p := storageEvent.Payload
		event := ListEvent{
			Source:       strings.SplitN(storageEvent.EventType, ".", 2)[0],
			ID:           storageEvent.MessageID,
			Type:         storageEvent.EventType,
			Time:         p.EventTime,
			ResourceId:   storageEvent.Payload.Target.ID,
			ResourceType: storageEvent.Payload.Target.TypeURI,
		}
		err := copier.Copy(&event.Initiator, &storageEvent.Payload.Initiator)
		if err != nil {
			return nil, err
		}

		if viper.GetBool("hermes.enrich_keystone_events") {
			nameMap := namesForIds(keystoneDriver, map[string]string{
				"init_user_domain":  event.Initiator.DomainID,
				"init_user_project": event.Initiator.ProjectID,
				"init_user":         event.Initiator.UserID,
				"target":            event.ResourceId,
			}, event.ResourceType)

			event.Initiator.DomainName = nameMap["init_user_domain"]
			event.Initiator.ProjectName = nameMap["init_user_project"]
			event.Initiator.UserName = nameMap["init_user"]
			event.ResourceName = nameMap["target"]
		}
		events = append(events, &event)
	}
	return events, nil
}

// GetEvent returns the CADF detail for event with the specified ID
func GetEvent(eventID string, tenantId string, keystoneDriver identity.Identity, eventStore storage.Storage) (*storage.EventDetail, error) {
	event, err := eventStore.GetEvent(eventID, tenantId)

	if viper.GetBool("hermes.enrich_keystone_events") {
		if event != nil {
			nameMap := namesForIds(keystoneDriver, map[string]string{
				"init_user_domain":  event.Payload.Initiator.DomainID,
				"init_user_project": event.Payload.Initiator.ProjectID,
				"init_user":         event.Payload.Initiator.UserID,
				"target":            event.Payload.Target.ID,
				"project":           event.Payload.Project,
				"user":              event.Payload.User,
				"group":             event.Payload.Group,
				"role":              event.Payload.Role,
			}, event.Payload.Target.TypeURI)

			event.Payload.Initiator.DomainName = nameMap["init_user_domain"]
			event.Payload.Initiator.ProjectName = nameMap["init_user_project"]
			event.Payload.Initiator.UserName = nameMap["init_user"]
			event.Payload.Target.Name = nameMap["target"]
			event.Payload.ProjectName = nameMap["project"]
			event.Payload.UserName = nameMap["user"]
			event.Payload.GroupName = nameMap["group"]
			event.Payload.RoleName = nameMap["role"]
		}
	}
	return event, err
}

// No Logic here, but handles mock implementation for eventStore
func GetAttributes(queryName string, tenantId string, eventStore storage.Storage) ([]string, error) {
	attribute, err := eventStore.GetAttributes(queryName, tenantId)

	return attribute, err
}

func namesForIds(keystoneDriver identity.Identity, idMap map[string]string, targetType string) map[string]string {
	nameMap := map[string]string{}
	var err error

	// Now add the names for IDs in the event to the nameMap
	iUserDomainId := idMap["init_user_domain"]
	if iUserDomainId != "" {
		nameMap["init_user_domain"], err = keystoneDriver.DomainName(iUserDomainId)
		if err != nil {
			log.Printf("Error looking up domain name for domain '%s'", iUserDomainId)
		}
	}
	iUserProjectId := idMap["init_user_project"]
	if iUserProjectId != "" {
		nameMap["init_user_project"], err = keystoneDriver.ProjectName(iUserProjectId)
		if err != nil {
			log.Printf("Error looking up project name for project '%s'", iUserProjectId)
		}
	}
	iUserId := idMap["init_user"]
	if iUserId != "" {
		nameMap["init_user"], err = keystoneDriver.UserName(iUserId)
		if err != nil {
			log.Printf("Error looking up user name for user '%s'", iUserId)
		}
	}
	projectId := idMap["project"]
	if projectId != "" {
		nameMap["project"], err = keystoneDriver.ProjectName(projectId)
		if err != nil {
			log.Printf("Error looking up project name for project '%s'", projectId)
		}
	}
	userId := idMap["user"]
	if userId != "" {
		nameMap["user"], err = keystoneDriver.UserName(userId)
		if err != nil {
			log.Printf("Error looking up user name for user '%s'", userId)
		}
	}
	groupId := idMap["group"]
	if groupId != "" {
		nameMap["group"], err = keystoneDriver.GroupName(groupId)
		if err != nil {
			log.Printf("Error looking up user name for group '%s'", groupId)
		}
	}
	roleId := idMap["role"]
	if roleId != "" {
		nameMap["role"], err = keystoneDriver.RoleName(roleId)
		if err != nil {
			log.Printf("Error looking up user name for role '%s'", roleId)
		}
	}

	// Depending on the type of the target, we need to look up the name in different services
	switch targetType {
	case "data/security/project":
		nameMap["target"], err = keystoneDriver.ProjectName(idMap["target"])
	case "service/security/account/user":
	// doesn't work for users - a UUID is used for some reason, which can't be looked up
	//	nameMap["target"], err = keystoneDriver.UserName(idMap["target"])
	default:
		log.Printf("Unhandled payload type \"%s\", cannot look up name.", targetType)
	}
	if err != nil {
		log.Printf("Error looking up name for %s '%s'", targetType, idMap["target"])
	}

	return nameMap
}
