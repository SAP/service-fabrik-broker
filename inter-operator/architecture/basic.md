# Service Fabrik Inter-operator Basic Architecture

##  Abstract

This document describes the basic architecture and scope for the Service Fabrik inter-operator. This includes the details about how it integrates with [Service Manager](https://github.com/Peripli/service-manager) on the one side and with the individual service [operators](https://coreos.com/operators/) on the other. This also includes some details about different possible Kubernetes cluster landscapes for hosting the Kubernetes-based services and how they can be managed.

## Target Audience

Architects, Developers, Product Owners, Development Managers who are interested in understanding/using Service Fabrik inter-operator to expose Kubernetes-based services as [OSB](https://www.openservicebrokerapi.org/)-compliant service brokers and integrate with [Service Manager](https://github.com/Peripli/service-manager).

## Table of Content
* [Service Fabrik Inter\-operator Basic Architecture](#service-fabrik-inter-operator-basic-architecture)
  * [Abstract](#abstract)
  * [Target Audience](#target-audience)
  * [Table of Content](#table-of-content)
  * [Context](#context)
  * [Integration with Service Manager](#integration-with-service-manager)
    * [Service Fabrik Inter\-operator Broker](#service-fabrik-inter-operator-broker)
    * [Service Fabrik Inter\-operator Provisioner](#service-fabrik-inter-operator-provisioner)
  * [Basic Control\-flow](#basic-control-flow)
    * [Catalog](#catalog)
      * [Service and Plan registration](#service-and-plan-registration)
      * [Service Fabrik Broker Catalog Cache](#service-fabrik-inter-operator-broker-catalog-cache)
      * [Integration with Service Manager](#integration-with-service-manager-1)
    * [Provision](#provision)
      * [Service Fabrik Inter\-operator Broker](#service-fabrik-inter-operator-broker-1)
      * [Service Fabrik Inter\-operator Provisioner](#service-fabrik-inter-operator-provisioner-1)
      * [Service Operator](#service-operator)
    * [Last Operation](#last-operation)
      * [Service Operator](#service-operator-1)
      * [Service Fabrik Inter\-operator Provisioner](#service-fabrik-inter-operator-provisioner-2)
      * [Service Fabrik Inter\-operator Broker](#service-fabrik-inter-operator-broker-2)
    * [Bind](#bind)
      * [Service Fabrik Inter\-operator Broker](#service-fabrik-inter-operator-broker-3)
      * [Service Fabrik Inter\-operator Provisioner](#service-fabrik-inter-operator-provisioner-3)
      * [Service Operator](#service-operator-2)
  * [Service Fabrik Inter\-operator Custom Resources](#service-fabrik-inter-operator-custom-resources)
    * [SFService](#sfservice)
    * [SFPlan](#sfplan)
      * [Templates](#templates)
        * [Template Variables](#template-variables)
        * [Actions](#actions)
        * [Types](#types)
        * [Remote Templates](#remote-templates)
        * [In\-line templates](#in-line-templates)
    * [SFServiceInstance](#sfserviceinstance)
      * [Rationale behind introducing the SFServiceInstance resource](#rationale-behind-introducing-the-sfserviceinstance-resource)
    * [SFServiceBinding](#sfservicebinding)
  * [Multi-Cluster provisioning Support for Interoperator](#multi-cluster-provisioning-support-for-interoperator)
    * [Why Multi Cluster Support is needed](#why-multi-cluster-support-is-needed)
    * [New Custom Resources Introduced](#new-custom-resources-introduced)
      * [SFCluster](#sfcluster)
    * [Components within Interoperator](#components-within-interoperator)
      * [Broker](#broker)
      * [MultiClusterDeployer](#multiclusterdeployer)
      * [Schedulers](#schedulers)
        * [DefaultScheduler](#defaultscheduler)
        * [Round Robin Scheduler](#roundrobinscheduler)
      * [Provisioner](#provisioner)
    * [Deployment Flow](#deployment-flow)
    * [Runtime Flow](#runtime-flow)
    * [Limitations with Multi-Cluster deployment](#limitations-with-multi-cluster-deployment)


## Context

The high-level approach recommendation for developing stateful services natively on Kubernetes is for the individual services to package their service implementation (including automated life-cycle activities) as a [Kubernetes Operator](https://coreos.com/operators/).
An operator is a combination of a set of [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) in Kubernetes and a set of custom controllers which watch, manage and implement a control-loop to take the required action to reconcile the desired state (as specified in the custom resources) with the actual state.

Typically, the operators are expected to manage their services within a given Kubernetes cluster and be feature-complete (via their [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) in the functionality they provide.

## Integration with Service Manager

[Service Manager](https://github.com/Peripli/service-manager) is a central repository of service brokers and platforms. It integrates with individual service brokers based on the [OSB](https://www.openservicebrokerapi.org/) API standard.

The guideline for developing stateful Kubernetes-native services is to develop a [Kubernetes Operator](https://coreos.com/operators/) for the service. This makes it very close to the paradigm of service development on Kubernetes as provide a powerful way to encapsulate both service and life-cycle functionality in once package.

This makes it necessary to bridge the gap between the Kubernetes [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)-based API of the operators with the [OSB](https://www.openservicebrokerapi.org/) API expected by the [Service Manager](https://github.com/Peripli/service-manager).

The inter-operator proposes to bridge this gap using a metadata-based approach and avoid too much of coding for this integration. The following metadata needs to be captured for a given operator so that it can be integrated as an OSB-compatible Service Broker with ServiceManager.

1. OSB Service and Service Plans that are supported by the operator.
1. Templates of the Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) of the operator.
1. Mapping of OSB actions such as `provision`, `deprovision`, `bind`, `unbind` etc. to the templated of Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) of the operator.

![Inter-operator Design](images/inter-operator.png)

### Service Fabrik Inter-operator Broker

The Service Fabrik Broker would act as the OSB API Adapter and is the component that integrates with the Service Manager. It is a lean component that serves OSB API requests and records the requests in a set of OSB-equivalent custom resources [`SFServiceInstance`](#sfserviceinstance) and [`SFServiceBinding`](#sfservicebinding).

These custom resources capture all the data sent in their corresponding OSB requests and act as a point of co-ordination between the inter-operator component that would then work to reconcile these OSB resources with the actual operator [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) based on the templates supplied in the catalog resources [`SFService`](#sfservice) and [`SFPlan`](#sfplan).

### Service Fabrik Inter-operator Provisioner

The inter-operator provisioner is a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) that keeps a watch on the [`SFServiceInstance`](#sfserviceinstance) and [`SFServiceBinding`](#sfservicebinding) custom resources and take the actions required as described [below]() to reconcile the corresponding resources of the service operator.

## Basic Control-flow

### Catalog

![Service Fabrik Inter-operator Basic Control-flow Catalog](images/basic-control-flow-catalog.png)

#### Service and Plan registration

The following steps are part of the landscape setup and a landscape administrator.
This could be an actual person or could be an automated component in itself.

1. Register `SFService` for the service.
There would be one `SFService` instance per service in the landscape.

It could be possible that a single Service Fabrik inter-operator serves multiple services in the same set of Kubernetes clusters. In such a case, there could be multiple `sfservices` registered for the same Service Fabrik inter-operator. But each of these `sfservices` would be for different individual services.

2. Register `sfplans` for each plan supported by the service.
As part of the away from t-shirt size approach to plans, it is recommended to minimize the number of plans per service. Ideally, that would be exactly one `SFPlan` per individual service.

Updates to the services and plans can be done as simple updates to the corresponding `sfservices` and `sfplans`. Service and plans can be unregistered by simply deleting the corresponding `sfservices` and `sfplans`.

TODO Backward compatibility existing instances must be handled by the individual service implementations and the applications properly.

#### Service Fabrik Broker Catalog Cache

The Service Fabrik Broker watches for registered `sfservices` and `sfplans`. It reacts to registrations, updates and deregistrations and keeps an up-to-date representation of the information.

#### Integration with Service Manager

1. An OSB client queries the [Service Manager](https://github.com/Peripli/service-manager) for a catalog of the available services via the `v2/catalog` request.
1. The Service Manager forwards this call (via some possible intermediaries) to the Service Fabrik Broker. 
1. The Service Fabrik Broker refers to its [internal up-to-date representation](#service-fabrik-broker-catalog-cache) and serves the catalog for the currently registered services.

### Provision

This section presumes that the `SFService` and `sfplans` are already registered as describe [above](#catalog).

![Service Fabrik Inter-operator Basic Control-flow Provision](images/basic-control-flow-provision.png)

#### Service Fabrik Inter-operator Broker

1. An OSB client makes a `provision` call to the [Service Manager](https://github.com/Peripli/service-manager).
1. The Service Manager forwards the call (perhaps via some intermediaries) to Service Fabrik Broker if the `provision` call was for a service and plan that was published by the Service Fabrik Broker.
The Service Manager adds some relevant additional context into the request.
1. The Service Fabrik Broker creates an `SFServiceInstance` capturing all the details passed in the `provision` request from the Service Manager.
The Service Fabrik Broker returns an asynchronous response.

#### Service Fabrik Inter-operator Provisioner

1. The inter-operator provisioner watches for `sfserviceinstances` and notices a newly created `SFServiceInstance`.
1. It loads the correct `provision` action template from the `SFPlan` corresponding to the `SFServiceInstance`.
1. It renders and applies the rendered template and creates the individual service's resources as specified in the template.

#### Service Operator

1. The individual service operator watches for its own Kubernetes API resources and notices a newly created set of resources.
1. It takes the required action to create the service instance.
1. It updates its Kubernetes API resources to reflect the status.

### Last Operation

This section presumes the following steps have already been performed.

1. `SFService` and `sfplans` are already registered as describe [above](#catalog).
1. A service instance is `provision`ed as described [above](#provision).

![Service Fabrik Inter-operator Basic Control-flow Last Operator](images/basic-control-flow-last-operation.png)

#### Service Operator

1. The individual service operator watches for its own Kubernetes API resources as well as all the lower level resources it has created to provision the service instance.
1. It notices a change in the status of any of the lower level resources and checks if the change in status is significant enough to be propagated to one of its own Kubernetes API resources.
1. It updates its corresponding Kubernetes API resources.

#### Service Fabrik Inter-operator Provisioner

1. The inter-operator provisioner watches for `sfserviceinstances` and the individual service operator's Kubernetes API resources (created using the `provision` template). It notices that some of the resources have been updated.
1. It uses the `status` template to extract the status information relevant to be propagated to the `SFServiceInstance`.
1. It updates the `SFServiceInstance`'s `status`.

#### Service Fabrik Inter-operator Broker

1. An OSB client makes a `last_operation` call to the [Service Manager](https://github.com/Peripli/service-manager).
1. The Service Manager forwards the call (perhaps via some intermediaries) to Service Fabrik Broker if the `provision` call was for a service instance that was provisioned by the Service Fabrik Broker.
The Service Manager adds some relevant additional context into the request.
1. The Service Fabrik Broker checks the `status` section of the `SFServiceInstance` and responds with the corresponding status.

### Bind

This section presumes the following steps have already been performed.

1. `SFService` and `sfplans` are already registered as describe [above](#catalog).
1. A service instance is `provision`ed as described [above](#provision).

![Service Fabrik Inter-operator Basic Control-flow Bind](images/basic-control-flow-bind.png)

#### Service Fabrik Inter-operator Broker

1. An OSB client makes a `bind` call to the [Service Manager](https://github.com/Peripli/service-manager).
1. The Service Manager forwards the call (perhaps via some intermediaries) to Service Fabrik Broker if the `bind` call was for a service, plan and the instance that was provisioned by the Service Fabrik Broker.
The Service Manager adds some relevant additional context into the request.
1. The Service Fabrik Broker creates an `SFServiceBinding` capturing all the details passed in the `bind` request from the Service Manager.
The Service Fabrik Broker returns an asynchronous response.

#### Service Fabrik Inter-operator Provisioner

1. The inter-operator provisioner watches for `sfservicebindings` and notices a newly created `SFServiceBinding`.
1. It loads the correct `bind` action template from the `SFPlan` corresponding to the `SFServiceBinding`.
1. It renders and applies the rendered template and creates the individual service's resources as specified in the template.

#### Service Operator

1. The individual service operator watches for its own Kubernetes API resources and notices a newly created set of resources.
1. It takes the required action to create the service instance.
1. It updates its Kubernetes API resources to reflect the status.

The binding response would follow a flow similar to the [`last_operation](#last-operation) flow above.

## Service Fabrik Inter-operator Custom Resources

The following custom resources are introduced as part of the Service Fabrik inter-operator to integrate with [Service Manager](https://github.com/Peripli/service-manager) on the one side and with the individual service [operators](https://coreos.com/operators/) on the other.

### SFService

The [`SFService`](../../config/crds/osb_v1alpha1_sfservice.yaml) captures the catalog/manifest details of an [`OSB Service`](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#service-offering-object) according to what is required to be served as part of the response for the `/v2/catalog` request.

For example,
```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFService
metadata:
  # Name maps to the name of the OSB Service.
  name: &id '24731fb8-7b84-5f57-914f-c3d55d793dd4'
spec:
  # Name of the OSB Service.
  name: &name postgresql

  # Id of the OSB Service.
  id: *id

  # Description of the OSB Service.
  description: &description 'Postgresql for internal development, testing, and documentation purposes of the Service Fabrik'

  # The following details map one-to-one with the data in the OSB service offering objects in the OSB /v2/catalog response.
  tags:
  - 'postgresql'
  requires: []
  bindable: true
  instanceRetrievable: true
  bindingRetrievable: true
  metadata:
    displayName: 'PostgreSQL'
    longDescription: *description
    providerDisplayName: 'SAP SE'
    documentationUrl: 'https://sap.com/'
    supportUrl: 'https://sap.com/'
  dashboardClient:
    id: postgresql-dashboard-client-id
    secret: postgresql-dashboard-client-secret
    redirectURI: 'https://sap.com/'
  planUpdatable: true

  # The following details are context input for Service Fabrik and the individual service operators.
  context:
    serviceFabrik:
      backupEnabled: false
    operator:
      image: "servicefabrikjenkins/blueprint"
      tag: "latest"
      port: 8080

```

The Service Fabrik Broker, as a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers), keeps a watch on `sfservices` and serves the subsequent `/v2/catalog` request according to the `sfservices` objects maintained as of the time of the request.

An operator can register one or more `sfservices`.

Deregistration of `sfservices` is handled using Kubernetes [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).

### SFPlan

The [`SFPlan`](../../config/crds/osb_v1alpha1_sfplan.yaml) captures the catalog/manifest details of an [`OSB Service Plan`](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#service-plan-object) according to what is required to be served as part of the response for the `/v2/catalog` request.

For example,
```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFPlan
metadata:
  # Name maps to the name of the OSB Service Plan.
  name: &id 29d7d4c8-6fe2-4c2a-a5ca-a826937d5a88
spec:
  # Name of the OSB Service Plan.
  name: &name 'v9.4-dev-large'

  # Id of the OSB Service Plan.
  id: *id

  # Description of the OSB Service Plan.
  description: 'Postgresql service running inside a k8s container (for non-productive usage)'

  # service_id of the OSB service to which this plan belongs.
  serviceId: '24731fb8-7b84-5f57-914f-c3d55d793dd4'

  # schemas describe the schema for the supported parameter for the provision and bind OSB actions.
  schemas:
    service_instance:
      create:
        parameters:
          "$schema": "http://json-schema.org/draft-06/schema#"
          title: createServiceInstance
          type: object
          additionalProperties: false
          properties:
            foo:
              type: string
              description: some description for foo field
          required:
          - "foo"

  # The following details map one-to-one with the data in the OSB service plan objects in the OSB /v2/catalog response.
  metadata:
    service-inventory-key: SERVICE-161
    costs:
    - amount:
        usd: 0.0
      unit: 'MONTHLY'
    bullets:
    - 'Container Deployment'
    - '128 MB Memory'
    - '100 MB Disk'
  free: true
  bindable: true
  planUpdatable: false

  # This section is configuration for to the operator and Service Fabrik.
  manager:
    async: true
  context:
    operator:
      image: "servicefabrikjenkins/blueprint"
      tag: "latest"
      port: 8080
      memory: '128m'
      persistent_volumes:
      - name: 'data'
        path: '/data'
        size: '100m'
    serviceFabrik:
      backupEnabled: false

  # templates map the OSB actions to the templates of the custom resources of the operator.
  templates:
  - action: provision
    type: gotemplate
    content: |-
      {{- $name := "" }}
      {{- with .instance.metadata.name }} {{ $name = . }} {{ end }}
      apiVersion: kubedb.com/v1alpha1
      kind: Postgres
      metadata:
      name: kdb-{{ $name }}-pg
      spec:
        version: 10.2-v1
        storageType: Durable
        storage:
          storageClassName: default
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 50Mi
        terminationPolicy: WipeOut

```

The Service Fabrik Broker, as a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers),
keeps a watch on `sfplans` and serves the subsequent `/v2/catalog` request according to the `sfplanss` objects maintained as of the time of the request.

An operator can register one or more `sfplans`.

Deregistration of `sfplans` is handled using Kubernetes [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).

#### Templates

Service Fabrik inter-operator's provisioner, currently, assumes that API of the individual service's operator would be Kubernetes Resources.
Service Fabrik inter-operator provisioner does not make any assumptions about the individual service operator's API apart from this.
Usually, they would be some [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
which would give the service operator implementation the full flexibility to implement and expose their functionality.

To enable this independence of API for the service operators, Service Fabrik inter-operator provisioner relies on the templates supplied in the [`sfplans`](#sfplan) to map the OSB actions to the specific CRDs or the individual service operators.

##### Template Variables

To provide the flexibility to the individual service implementations, many standard template variables are supplied during the rendering of the templates.

At a minimum, the following variable would be supported.
1. `SFService`.
1. `SFPlan`.
1. `SFInstance`.
1. `SFBind` for `bind` request.

More variables such as the actual resources created by the template might also be made available in the future.

##### Actions

The `action` field can be used to specify the OSB action for which the template supplied is applicable. Typically, these would include `provision`, `bind` etc. But these could be extended to custom/generic actions.

##### Types

The `type` field can be used to specify the type of template itself. For example, [`gotemplate`](https://golang.org/pkg/text/template/), [`helm`](https://helm.sh/) etc. In future, additional template types could be supported such as [`jsonnet`](https://jsonnet.org/).

Refer [here](./gotemplate.md) for details on additional functions provided by interoperator along with `gotemplate`. Currently, only a single resource is expected to be generated by the `gotemplates`. The type `helm` supports the generation of multiple resources.

##### Remote Templates

The `url` field can be used to specify the location where the actual templates can be found. For example,

```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFPlan
spec:
  templates:
  - action: provision
    type: gotemplate
    url: "https://raw.githubusercontent.com/cloudfoundry-incubator/service-fabrik-broker/feature/inter-operator/interoperator/config/samples/templates/gotemplates/postgres/postgres.yaml"
```

Please note that the URLs have to be accessible for the Service Fabrik inter-operator. This is especially relevant in the private cloud scenario.

##### In-line templates

Since service operators are expected to [feature-complete](#context) in their API, it would be very common scenario that an OSB action maps to a single (possibly the same) Kubernetes resource of the service operator.
The template type `gotemplate` fits this use-case well.
This common use-case can be easily implemented by using the `content` field to specify the `gotemplate` content directly in-line in the `SFPlan` rather than referring to it in a remote location using the `url` field (which is also possible).

For example,

```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFPlan
spec:
templates:
  - action: provision
    type: gotemplate
    content: |-
      {{- $name := "" }}
      {{- with .instance.metadata.name }} {{ $name = . }} {{ end }}
      apiVersion: kubedb.com/v1alpha1
      kind: Postgres
      metadata:
      name: kdb-{{ $name }}-pg
      spec:
        version: 10.2-v1
        storageType: Durable
        storage:
          storageClassName: default
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 50Mi
        terminationPolicy: WipeOut
```

### SFServiceInstance

The [`SFServiceInstance`](../../config/crds/osb_v1alpha1_sfserviceinstance.yaml) captures all the details from an OSB `provision` request.

For example,
```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFServiceInstance
metadata:
  # Name would map to the instance_id from the OSB provision request.
  name: '0304b210-fcfd-11e8-a31b-b6001f10c97f'
spec:
  # service_id as in the OSB provision request.
  serviceId: '24731fb8-7b84-5f57-914f-c3d55d793dd4'

  # plan_id as in the OSB provision request.
  planId: '29d7d4c8-6fe2-4c2a-a5ca-a826937d5a88'

  # context contains all the data that is passed as part of the context in the OSB provision request.
  context:
    organizationGuid: organization-guid
    spaceGuid: space-guid

  # parameters as passed to the OSB provision request.
  parameters:

# status would be updated by the inter-operator.
status:
  state:
  dashboardUrl:

```

The inter-operator provisioner as a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) that keeps a watch on `sfserviceinstances` and take action as described [below]() to reconcile the actual operator [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

`Deprovision` is handled using Kubernetes [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).

#### Rationale behind introducing the `SFServiceInstance` resource

Technically, the functionality of the Service Fabrik inter-operator provisioner can be implemented without using the `SFServiceInstance` resource for simpler use-cases.
For example, in the [`provision`] control-flow, the Service Fabrik Broker can directly lookup the [`SFPlan`] and apply the right template and create the actual service-specific resources directly without having to create an intermediate `SFServiceIntance` resource first to be picked up by the `Service Fabrik inter-operator provisioner.
This might work well for the scenario where the Service Fabrik in provisioned on the same Kubernetes cluster as where the service operator and it's instances are also eventually provisioned.
But as discussed in the [cluster landscape document](cluster-landscape.md), there are [reasons](cluster-landscape.md#tree-dimensions-for-comparison) to [recommend](cluster-landscape.md#recommended-landscape-scenario) more dynamic scenarios involving multiple Kubernetes clusters where the Kubernetes cluster where Service Fabrik is provisioned would be different from the Kubernetes cluster where the service operator and the instances are provisioned.
This would lead to a [design](cluster-landscape.md#service-instance-scheduling) where there a scheduler to provide loose coupling between the scheduling decision (in which Kubernetes cluster a particular service instance is to be provisioned) and the actual details of provisioning.
Such a design would necessitate two sets of custom resources.
1. One resource on the Service Fabrik side on which the scheduling decision can be take an recorded.
1. Another resource (or set of resources) which are to be acted upon by the service operator.

In such a scenario, it makes sense to leverage the first resource on the Service Fabrik side to record the OSB request almost verbatim which leads to the current `SFServiceInstance` design.

### SFServiceBinding

The [`SFServiceBinding`](../../config/crds/osb_v1alpha1_sfservicebinding.yaml) captures all the details from an OSB `bind` request.

For example,
```yaml
apiVersion: osb.servicefabrik.io/v1alpha1
kind: SFServiceBinding
metadata:
  # Name would map to the binding_id from the OSB bind request.
  name: 'de3dd272-fcfc-11e8-a31b-b6001f10c97f'
spec:
  # instance_id as in the OSB bind request.
  instanceId: 0304b210-fcfd-11e8-a31b-b6001f10c97f

  # service_id as in the OSB bind request.
  serviceId: '24731fb8-7b84-5f57-914f-c3d55d793dd4'

  # plan_id as in the OSB bind request.
  planId: '29d7d4c8-6fe2-4c2a-a5ca-a826937d5a88'

  # bind_resource as in the OSB bind request.
  bindResource:

  # context contains all the data that is passed as part of the context in the OSB bind request.
  context:
    organizationGuid: organization-guid
    spaceGuid: space-guid

  # parameters as passed to the OSB bind request.
  parameters:
  
status:
  state:

```

The inter-operator provisioner as a [custom controller](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers) that keeps a watch on `sfservicebindings` and take action as described [below]() to reconcile the actual operator [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

`Unbind` is handled using Kubernetes [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).


# Multi-Cluster provisioning Support for Interoperator
Multi-cluster provisioning support enables provisioning and distribution of the service instances into multiple clusters. From the list of multiple clusters, one is selected based on the chosen scheduler and the `SFServiceInstance` updated with the `clusterId` value. The `SFServiceInstance` is then also copied to the cluster it is scheduled to. Every cluster should have the service operator already installed within it. The service fabrik inter-operator provisioner would then pick up the event generated by the creation of the `SFServiceInstnce` which in turn creates the service specific CRDs which service operator listens to.
## Why Multi Cluster Support is needed
Scalability is the main reason why one should use Multi-Cluster support. It gives you an option to add new clusters into your set of clusters and scale horizontally. There could be many limitations with the number of resources you can spawn in a cluster such as finite capacity of the worker nodes constraining the number of services that can be scheduled on a given worker node, some finite maximum number of nodes per cluster due to some constraints in the cluster control plane or infrastructure. Hence, for a production scenario, multi-cluster support will be required so that services can be scheduled and spread across multiple clusters and can be scaled horizontally.

Regarding the type of scheduling algorithms which are supported, we currently support round-robin and least-filled scheduler. We also plan to implement other schedulers which can be used. Schedulers are discussed later in the [schedulers](#schedulers) section.
## New Custom Resources Introduced
Along with the custom resources like `SFService`, `SFPlan`, `SFServiceInstance` and `SFServiceBinding` which are discussed earlier, we also introduce `SFCluster` as a new CRD.
### SFCluster
`SFCluster` is the CRD which stores the details of the cluster where service instances are to be provisioned. One `SFCluster` CRD instance must be maintained for each cluster that is onboarded for provisioning service instances. The structure of a sample resource look like the following.

```yaml
apiVersion: resource.servicefabrik.io/v1alpha1
kind: SFCluster
metadata:
  name: "1"
  namespace: interoperator
spec:
  secretRef: 1-kubeconfig
```
where the secretRef looks like the following

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: 1-kubeconfig
  namespace: interoperator
data:
  kubeconfig: <REDACTED_KUBECONFIG>
```
## Components within Interoperator
Below, we discuss about the components of Service Fabrik Interoperator. Some components like the broker and the provisioner were already introduced earlier. With Multi-Cluster deploy support, we bring in two new components, `MultiClusterDeployer` and `Scheduler` which are also described below.
### Broker
Broker was already introduced earlier, please read about it in the earlier section [here](#service-fabrik-inter-operator-broker)
### MultiClusterDeployer
This component is a set of [custom controllers](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers). Below are the list of controllers it comprises of.
#### Provisioner Controller
Provisioner Controller is the custom controller which watches on the `SFCluster` of the master cluster and deploys the [Provisioner](#provisioner) component in those clusters.
#### Service Replicator
Service Replicator is the custom controller which watches on the `SFClusters`, `SFServices` and `SFPlans` of the master cluster and copies the `SFServices` and `SFPlans` from master cluster to sister clusters.
#### Service Instance Reconciler
Service Instance Reconciler is the custom controller which watches across multiple clusters that are part of the cluster registry(set of `SFClusters`) and reconciles a `SFServiceInstance` between master cluster and its assigned cluster, assigned by [Scheduler](#schedulers).
#### Service Binding Reconciler
Service Binding Reconciler is the custom controller which watches across multiple clusters, part of the cluster registry(set of `SFClusters`) and reconciles a `SFServiceBinding` between master cluster and its assigned cluster, assigned by [Scheduler](#schedulers).
### Schedulers
Schedulers are basically custom controller running on master cluster watching on `SFServiceInstances` and schedules/assigns them `clusterId` (the name of the corresponding `SFCluster` instance) of the cluster where the instance need to be provisioned, depending on the scheduling algorithm it implements. We currently have implemented the following set of schedulers described below. Activating a scheduler is config driven to be passed when someone deploys Inter-operator.
#### DefaultScheduler
This is just a sample scheduler suitable only for the single cluster setup. In that case, it schedules all the instances in the one cluster which is part of the setup. It is not suitable for the multi-cluster setup.
#### Round Robin Scheduler
As the name suggests, round robin scheduler schedules instances in round robin fashion. At this point, it does not take care of capacity and if interoperator restarts, it loses the state about the next cluster to be scheduled and starts scheduling from the beginning. 
#### Least filled Scheduler
This scheduler schedules instance in the least filled cluster.
### Provisioner
Provisioner was also already introduced earlier, please read about it in the earlier section [here](#service-fabrik-inter-operator-provisioner). In the multi-cluster setup, provisioners are deployed across multiple clusters by interoperator automatically. More details can be found in the [deployment flow](#deployment-flow) section.
## Deployment Flow
Following are the flow for a deployment of Interoperator.
1. When Interoperator is deployed initially, one deploys the [broker](#broker), [MultiClusterDeployer](#multiclusterdeployer) and the [Scheduler](#schedulers) component in a cluster, called as master cluster.
2. After this, the operator should create the `SFServices`, `SFPlans` and `SFClusters` in the master cluster. `SFClusters` is simply the list/registry of all clusters where you want to provision the instances. We also refer to them as sister cluster interchangebly. Master cluster can also be part of the cluster registry and be a sister cluster in itself, if someone wants to use it for service provisioning as well.
3. [Provisioner Controller](#provisioner-controller) then takes care of replicating the provisioner component to all sister clusters and [Service Replicator](#service-replicator) takes care of replicating the SFServices and SFPlans in all the clusters.

Now the setup is ready for taking requests. We depict this in the picture below.
![Inter-operator Deployment Flow](images/Deployment%20Flow%20Updated.png)
## Runtime Flow
After the interoperator is ready and setup across multiple clusters as described [above](#deployment-flow), service instance and service binding can be created. When in the master cluster, broker creates an `SFServiceInstance`, Scheduler picks it up first and schedules/assigns a cluster where service needs to be provisioned. Then [Service Instance Reconciler](#service-instance-reconciler) reconciles that `SFServiceInstance` in the sister cluster where it is scheduled. Once that is done, [provisioner](#provisioner) residing in the sister cluster takes over and from then onwards, the process described in [service provisioning](#service-fabrik-inter-operator-provisioner-1) is followed. For another `SFServiceInstance`, it is again scheduled in one of the sister cluster and provisioner provisions the service there. The picture below describes the steps.
![Inter-operator Runtime Flow](images/Runtime%20Flow%20Updated.png)

## Limitations with Multi-Cluster deployment
1. Interoperator currently does not take care of the cluster off-boarding.
2. Service Operator in each sister cluster is assumed to be already deployed and its version update/upgrade is managed/maintained by the service operator. Inter-operator does not do anything about it.
3. Interoperator does not take care of the Kubernetes and OS updates to the onboarded clusters.
4. Service owners will have to monitor the clusters and their resource situations and add additional sister clusters if required.

