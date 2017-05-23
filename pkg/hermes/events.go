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
	"github.com/jinzhu/copier"
	"github.com/sapcc/hermes/pkg/keystone"
	"github.com/sapcc/hermes/pkg/storage"
	"github.com/sapcc/hermes/pkg/util"
	"log"
	"strings"
)

// ListEvent contains high-level data about an event, intended as a list item
type ListEvent struct {
	Source       string `json:"source"`
	ID           string `json:"event_id"`
	Type         string `json:"event_type"`
	Time         string `json:"event_time"`
	ResourceName string `json:"resource_name"`
	ResourceId   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	Initiator    struct {
		TypeURI     string `json:"typeURI"`
		DomainID    string `json:"domain_id,omitempty"`
		DomainName  string `json:"domain_name,omitempty"`
		ProjectID   string `json:"project_id,omitempty"`
		ProjectName string `json:"project_name,omitempty"`
		UserID      string `json:"user_id"`
		UserName    string `json:"user_name"`
		Host        struct {
			Agent   string `json:"agent"`
			Address string `json:"address"`
		} `json:"host"`
		ID string `json:"id"`
	} `json:"initiator"`
}

// Filter maps to the filtering/paging/sorting allowed by the API
type Filter struct {
	Source       string
	ResourceType string
	ResourceName string
	UserName     string
	EventType    string
	Time         string
	Offset       uint64
	Limit        uint64
	Sort         string
}

// GetEvents returns a list of matching events (with filtering)
func GetEvents(filter *Filter, tenantId string, keystoneDriver keystone.Driver, eventStore storage.Driver) ([]*ListEvent, int, error) {
	storageFilter := storageFilter(filter, keystoneDriver)

	util.LogDebug("hermes.GetEvents: tenant id is %s", tenantId)
	eventDetails, total, err := eventStore.GetEvents(&storageFilter, tenantId)
	if err != nil {
		return nil, 0, err
	}
	events, err := eventsList(eventDetails, keystoneDriver)
	if err != nil {
		return nil, 0, err
	}
	return events, total, err
}

func storageFilter(filter *Filter, keystoneDriver keystone.Driver) storage.Filter {
	// As per the documentation, the default limit is 10
	if filter.Limit == 0 {
		filter.Limit = 10
	}
	// TODO: Check storage driver for max limit

	storageFilter := storage.Filter{
		Source:       filter.Source,
		ResourceType: filter.ResourceType,
		EventType:    filter.EventType,
		Time:         filter.Time, // TODO: This will probably get more complicated...
		Offset:       filter.Offset,
		Limit:        filter.Limit,
		Sort:         filter.Sort, // TODO: This will probably get more complicated...
	}
	// Translate hermes.Filter to storage.Filter by filling in IDs for names
	if filter.ResourceName != "" {
		// TODO: make sure there is a resource type, then look up the corresponding name
		//storageFilter.ResourceId = resourceId
	}
	if filter.UserName != "" {
		userId, err := keystoneDriver.UserId(filter.UserName)
		if err != nil {
			util.LogError("Could not find user ID for name %s", filter.UserName)
		}
		storageFilter.UserId = userId
	}
	return storageFilter
}

// Construct ListEvents and add the names for IDs in the events
func eventsList(eventDetails []*storage.EventDetail, keystoneDriver keystone.Driver) ([]*ListEvent, error) {
	var events []*ListEvent
	for _, storageEvent := range eventDetails {
		p := storageEvent.Payload
		event := ListEvent{
			Source:       strings.SplitN(storageEvent.EventType, ".", 2)[0],
			ID:           p.ID,
			Type:         storageEvent.EventType,
			Time:         p.EventTime,
			ResourceId:   storageEvent.Payload.Target.ID,
			ResourceType: storageEvent.Payload.Target.TypeURI,
		}
		err := copier.Copy(&event.Initiator, &storageEvent.Payload.Initiator)
		if err != nil {
			return nil, err
		}

		nameMap := namesForIds(keystoneDriver, map[string]string{
			"domain":  event.Initiator.DomainID,
			"project": event.Initiator.ProjectID,
			"user":    event.Initiator.UserID,
			"target":  event.ResourceId,
		}, event.ResourceType)

		event.Initiator.DomainName = nameMap["domain"]
		event.Initiator.ProjectName = nameMap["project"]
		event.Initiator.UserName = nameMap["user"]
		event.ResourceName = nameMap["target"]

		events = append(events, &event)
	}
	return events, nil
}

// GetEvent returns the CADF detail for event with the specified ID
func GetEvent(eventID string, tenantId string, keystoneDriver keystone.Driver, eventStore storage.Driver) (*storage.EventDetail, error) {
	event, err := eventStore.GetEvent(eventID, tenantId)

	if event != nil {
		nameMap := namesForIds(keystoneDriver, map[string]string{
			"domain":  event.Payload.Initiator.DomainID,
			"project": event.Payload.Initiator.ProjectID,
			"user":    event.Payload.Initiator.UserID,
			"target":  event.Payload.Target.ID,
		}, event.Payload.Target.TypeURI)

		event.Payload.Initiator.DomainName = nameMap["domain"]
		event.Payload.Initiator.ProjectName = nameMap["project"]
		event.Payload.Initiator.UserName = nameMap["user"]
		event.Payload.Target.Name = nameMap["target"]
	}
	return event, err
}

func namesForIds(keystoneDriver keystone.Driver, idMap map[string]string, targetType string) map[string]string {
	nameMap := map[string]string{}
	var err error

	// Now add the names for IDs in the event
	domainId := idMap["domain"]
	if domainId != "" {
		nameMap["domain"], err = keystoneDriver.DomainName(domainId)
		if err != nil {
			log.Printf("Error looking up domain name for domain '%s'", domainId)
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

	// Depending on the type of the target, we need to look up the name in different services
	switch targetType {
	case "data/security/project":
		nameMap["target"], err = keystoneDriver.ProjectName(idMap["target"])
	case "service/security/account/user":
		nameMap["target"], err = keystoneDriver.UserName(idMap["target"])
	default:
		log.Printf("Unhandled payload type \"%s\", cannot look up name.", targetType)
	}
	if err != nil {
		log.Printf("Error looking up name for %s '%s'", targetType, userId)
	}

	return nameMap
}
