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

package storage

// Storage is an interface that wraps the underlying event storage mechanism.
// Because it is an interface, the real implementation can be mocked away in unit tests.
type Storage interface {

	/********** requests to ElasticSearch **********/
	GetEvents(filter *Filter, tenantId string) ([]*EventDetail, int, error)
	GetEvent(eventId string, tenantId string) (*EventDetail, error)
	GetAttributes(queryName string, tenantId string) ([]string, error)
	MaxLimit() uint
}

// FieldOrder maps the sort Fieldname and Order
type FieldOrder struct {
	Fieldname string
	Order     string //asc or desc
}

// This Filter is similar to hermes.Filter, but using IDs instead of names
type Filter struct {
	Source       string
	ResourceType string
	ResourceId   string
	UserId       string
	EventType    string
	Time         map[string]string
	Offset       uint
	Limit        uint
	Sort         []FieldOrder
}

// Thanks to the tool at https://mholt.github.io/json-to-go/

//  The JSON annotations are for parsing the result from ElasticSearch
type eventListWithTotal struct {
	Total  int           `json:"total"`
	Events []EventDetail `json:"events"`
}

// EventDetail contains the CADF payload, enhanced with names for IDs
//  The JSON annotations are for parsing the result from ElasticSearch AND for generating the Hermes API response
type EventDetail struct {
	PublisherID string `json:"publisher_id"`
	EventType   string `json:"event_type"`
	Payload     struct {
		Observer struct {
			TypeURI string `json:"typeURI"`
			ID      string `json:"id"`
		} `json:"observer"`
		ResourceInfo string `json:"resource_info"`
		TypeURI      string `json:"typeURI"`
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
		EventTime   string `json:"eventTime"`
		Action      string `json:"action"`
		EventType   string `json:"eventType"`
		ID          string `json:"id"`
		Outcome     string `json:"outcome"`
		Role        string `json:"role,omitempty"`
		RoleName    string `json:"role_name,omitempty"`
		Project     string `json:"project,omitempty"`
		ProjectName string `json:"project_name,omitempty"`
		User        string `json:"user,omitempty"`
		UserName    string `json:"user_name,omitempty"`
		Group       string `json:"group,omitempty"`
		GroupName   string `json:"group_name,omitempty"`
		Target      struct {
			TypeURI string `json:"typeURI"`
			ID      string `json:"id"`
			Name    string `json:"name,omitempty"`
		} `json:"target"`
	} `json:"payload"`
	MessageID string `json:"message_id"`
	Priority  string `json:"priority"`
	Timestamp string `json:"timestamp"`
}

type AttributeValueList []AttributeValue

type AttributeValue struct {
	Value string `json:"value"`
	count int64  `json:"count"` // Removing export due to desire to not include it in JSON return
}
