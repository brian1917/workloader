package awslabel

import "github.com/aws/aws-sdk-go/service/ec2"

type AwsCLIResponse struct {
	Reservations []struct {
		ReservationId string         `json:"ReservationId"`
		OwnerId       string         `json:"OwnerId"`
		Groups        []interface{}  `json:"Groups"`
		Instance      []ec2.Instance `json:"Instances"`
	}
}

type Tags map[string]string
