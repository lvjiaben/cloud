module lina-plugin-manager-contract-tests

go 1.25.0

require (
 lina-core v0.0.0
 lina-plugin-idc-core v0.0.0
 lina-plugin-idc-resource v0.0.0
 lina-plugin-idc-payment v0.0.0
 lina-plugin-idc-portal v0.0.0
 gopkg.in/yaml.v3 v3.0.1
)

replace lina-core => ../../../../../lina-core

replace lina-plugin-idc-core => ../../../../idc-core

replace lina-plugin-idc-resource => ../../../../idc-resource

replace lina-plugin-idc-payment => ../../../../idc-payment

replace lina-plugin-idc-portal => ../../../../idc-portal
