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
	"github.com/sapcc/hermes/pkg/data"
	"github.com/sapcc/hermes/pkg/keystone"
	"github.com/sapcc/hermes/pkg/storage"
	"github.com/sapcc/hermes/pkg/util"
	"log"
)

// GetEvents returns a list of matching events (with filtering)
func GetEvents(filter *data.Filter, tenantId string, keystoneDriver keystone.Interface, eventStore storage.Interface) ([]*data.Event, int, error) {
	// As per the documentation, the default limit is 10
	if filter.Limit == 0 {
		filter.Limit = 10
	}

	util.LogDebug("hermes.GetEvents: tenant id is %s", tenantId)
	events, total, err := eventStore.GetEvents(*filter, tenantId)

	// Now add the names for IDs in the events
	for _, event := range events {
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
	}

	return events, total, err
}

// GetEvent returns the CADF detail for event with the specified ID
func GetEvent(eventID string, tenantId string, keystoneDriver keystone.Interface, eventStore storage.Interface) (*data.EventDetail, error) {
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

func namesForIds(keystoneDriver keystone.Interface, idMap map[string]string, targetType string) map[string]string {
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
