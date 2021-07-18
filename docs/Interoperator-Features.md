## Service Instance Sharing
This feature is tested with Cloud Foundry as consuming platform. Refer [this documentation](https://docs.cloudfoundry.org/devguide/services/sharing-instances.html) to know more background. Follow these steps to enable instance sharing for your service.
1. Set parameter `shareable: true` under `metadata` in your `SFService` CR. 
```shell
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFService
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: ...
  namespace: ....
spec:
  ...
  ...
  metadata:
    displayName: <Service Display Name>
    ...
    shareable: true
  ...
  dashboardClient:
    id: <service-dashboard-client-id>
  ...
```

Also ensure that the CF has updated catalog using `cf update-service-broker`.

2. Use `cf share-service` command of CF CLI to share the instance with given org and space. This command must be run from source space, i.e., the space where the service instance exists.
```shell
$ cf share-service <service-instance-name> -s OTHER-SPACE [-o OTHER-ORG] 
```
Note: It must be ensured that access to your service has been enabled in target space/org using `cf enable-service-access`.

3. [Optional] Your Service Operator must be modified to support instance sharing if necessary. (Please note that this might not be needed for all operators).

Now the apps running in target space should be able to bind service instances in source space.

## Service Instance Dashboard

If a service has instance specific dashboard, it is possible to send the custom dashboard url as part of the provision/update request. To leverage custom dashboard url template for the plans, under SFPlans, one needs to add the following metadata. 

```
spec
  manager:
    async: true
    settings:
      dashboard_url_template: JHtpbnN0YW5jZS5zcGVjLmNsdXN0ZXJJZCA9PSAxID8gJ2h0dHBzOi8vbXlkZWZhdWx0Y2x1c3Rlci0nK2luc3RhbmNlLm1ldGFkYXRhLm5hbWUrJy5teWRvbWFpbi5jb20vdWknIDogJ2h0dHBzOi8vJytpbnN0YW5jZS5zcGVjLmNsdXN0ZXJJZCsnLScraW5zdGFuY2UubWV0YWRhdGEubmFtZSsnLm15ZG9tYWluLmNvbS91aSd9

```

In this, the value of `dashboard_url_template` is a base64 encoded string of the url template.
Following object will be passed to the template for rendering
```
{
plan: <plan-in-catalog>,
instance: <sfserviceinstance cr>,
instance_id: <instance_id as part of osb req>
plan_id: <plan_id as part of osb req>,
service_id: <service_id as part of osb req>,
...other osb req body and params
}
```
For example, if the URL is of the form `https://${instance_id}.mydomain.com/ui`, then one has to base64 encode it and set it as the value. the expression ${instance_id} will be automatically evaluated. One can use more variables like `plan_id` `service_id` which are part of body or parameters of standard OSBAPI request.

The template also support more complex use cases with conditional checks. For example let's take a scenario when instances are being provisioned on multiple clusters, in that case the dashboard url will depend on cluster domain. One can leverage sfserviceinstance object which is passed to template to determine the cluster where the instance will be provisioned.

`${instance.spec.clusterId == 1 ? \'https://mydefaultcluster-\'+instance.metadata.name+\'.mydomain.com/ui\' : \'https://\'+instance.spec.clusterId+\'-\'+instance.metadata.name+\'.mydomain.com/ui\'}`

## Quota Management

Quota management can be enabled setting the quota related values mentioned [here](https://github.com/cloudfoundry-incubator/service-fabrik-broker/blob/master/helm-charts/interoperator/values.yaml#L16-L21) and enabling quota and other related details.

As a Service Provider, if one wants to define quotas on services provisioned, It can be possible using Interoperator. Here is how quota management works in Interoperator.


* If the quota check is turned on, then Interoperator does the quota check in the following way : 
  * It integrates with an entitlement service which is supposed to expose additional API that returns the quota assigned to a CF org(`GET /orgs/{orgId}/services/{serviceName}/plan/{versionIndependentPlanName}`) or a subaccount for a given service and service plan which returns the quantity as integer assigned to the given org/subaccount for the specified service and service plan name (should be version independent). The details of the entitlement service and co-ordinates are to be added in the values as mentioned before.
  Interoperator Broker calls this API when a new service instance is created.

  * Already used quota is calculated from the APIServer and does not need any external dependency like CF.

  * Remaining quota is calculated based on above two values and based on that provision request is allowed.

## Asynchronous Service Binding Operations

If the service supports, it is possible to perform service binding operations asynchronously. To leverage asynchronous service binding for the plans, under SFPlans, one needs to add the following metadata.

```
spec
  manager:
    async: true
    asyncBinding: false
```

If `asyncBinding` is set to `true` for the plan, the service binding operation will be asynchronous. The default value is `false` if omitted from the plan and the default behavior is synchronous. The `async` flag is used to control the behavior of service provisioning operations. The platform must include the query parameter `accepts_incomplete=true` in the request of [asynchronous operations](https://github.com/openservicebrokerapi/servicebroker/blob/v2.14/spec.md#asynchronous-operations).

One can use the [last operation endpoint for service bindings](https://github.com/openservicebrokerapi/servicebroker/blob/v2.14/spec.md#polling-last-operation-for-service-bindings) to poll the state of the service binding operation.
Refer [this documentation](https://github.com/openservicebrokerapi/servicebroker/blob/v2.14/spec.md#binding) to know more.

Please note that `cf create-service-key` operation does not support asynchronous service binding. So `cf create-service-key` will not work with asynchronous plans. 

## GET Endpoints for Service Instance and Service Binding

One can fetch a [service instance](https://github.com/openservicebrokerapi/servicebroker/blob/v2.14/spec.md#fetching-a-service-instance), if `instances_retrievable :true` is declared for the service in the Catalog.
The GET endpoint for a service instance is `/v2/service_instances/{instance_id}`.

To fetch a [service binding](https://github.com/openservicebrokerapi/servicebroker/blob/v2.14/spec.md#fetching-a-service-binding), `bindings_retrievable :true` must be declared for the service in the Catalog.
The GET endpoint for a service binding is `/v2/service_instances/{instance_id}/service_bindings/{binding_id}`.
```
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFService
spec:
  instancesRetrievable: true
  bindingsRetrievable: true
  ...
  ...
```
A minimum `X-Broker-API-Version` of 2.14 is required for the GET endpoints.

## Single Namespace for all Service Instances

The Interoperator creates separate namespaces for each service instances. If one wants all the service instance under a single namespace, set `broker.enable_namespaced_separation` as `false` in `values.yaml`.
If you are using helm, then
```sh
$ helm install --set cluster.host=xxxx \
     --set broker.enable_namespaced_separation=false \
     --set broker.services_namespace=services \
     --namespace interoperator --wait interoperator interoperator
```
