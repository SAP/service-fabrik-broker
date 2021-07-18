'use strict';

const _ = require('lodash');
const assert = require('assert');
const Promise = require('bluebird');
const config = require('@sf/app-config');
const decamelizeKeysDeep = require('decamelize-keys-deep');
const logger = require('@sf/logger');
const { catalog } = require('@sf/models');
const { apiServerClient } = require('../');
const {
  CONST,
  errors: { NotFound }
} = require('@sf/common-utils');

function getAllServices() {
  let namespaceOpts = {};
  if (_.get(config, 'sf_namespace')) {
    namespaceOpts.namespaceId = _.get(config, 'sf_namespace');
  } else {
    namespaceOpts.allNamespaces = true;
  }
  return apiServerClient.getResources(_.merge({
    resourceGroup: CONST.APISERVER.RESOURCE_GROUPS.INTEROPERATOR,
    resourceType: CONST.APISERVER.RESOURCE_TYPES.INTEROPERATOR_SERVICES
  },namespaceOpts))
    .then(serviceList => {
      let services = [];
      _.forEach(serviceList, service => {
        services = _.concat(services, [decamelizeKeysDeep(service.spec)]);
      });
      return services;
    });
}

function getAllPlansForService(serviceId) {
  let namespaceOpts = {};
  if (_.get(config, 'sf_namespace')) {
    namespaceOpts.namespaceId = _.get(config, 'sf_namespace');
  } else {
    namespaceOpts.allNamespaces = true;
  }
  return apiServerClient.getResources(_.merge({
    resourceGroup: CONST.APISERVER.RESOURCE_GROUPS.INTEROPERATOR,
    resourceType: CONST.APISERVER.RESOURCE_TYPES.INTEROPERATOR_PLANS,
    query: {
      labelSelector: `serviceId=${serviceId}`
    }
  }, namespaceOpts))
    .then(planList => {
      let plans = [];
      _.forEach(planList, plan => {
        plans = _.concat(plans, [plan.spec]);
      });
      return plans;
    });
}

function registerSFEventsCrd() {
  return Promise.all([
    apiServerClient.registerCrds(CONST.APISERVER.RESOURCE_GROUPS.INSTANCE, CONST.APISERVER.RESOURCE_TYPES.SFEVENT)
  ]);
}

function waitWhileCRDsAreRegistered() {
  // We assume here that the Helm chart based deployment or the pre-start script of interoperator job in case of BOSH based deployment will take care of registering the CRDs. While it is not registered, we wait!
  logger.info('Checking if the CRDs are already registered');
  return Promise.all([
    apiServerClient.getCustomResourceDefinition(CONST.APISERVER.RESOURCE_TYPES.INTEROPERATOR_SERVICES + '.' + CONST.APISERVER.RESOURCE_GROUPS.INTEROPERATOR),
    apiServerClient.getCustomResourceDefinition(CONST.APISERVER.RESOURCE_TYPES.INTEROPERATOR_PLANS + '.' + CONST.APISERVER.RESOURCE_GROUPS.INTEROPERATOR)
  ])
    .catch(NotFound, () => {
      logger.info('Waiting for the CRDs to get registered');
      return Promise.delay(CONST.APISERVER.WAIT_IN_MS_BEFORE_STARTUP).then(() => waitWhileCRDsAreRegistered());
    });
}

function getServiceCrdFromConfig(service) {
  assert.ok(service.name, 'service.name is required to generate plan crd');
  assert.ok(service.id, 'service.id is required to generate plan crd');
  assert.ok(service.description, 'service.description is required to generate plan crd');
  assert.ok(service.bindable, 'service.bindable is required to generate plan crd');

  let serviceCRD = {
    apiVersion: 'osb.servicefabrik.io/v1alpha1',
    kind: 'SFService',
    metadata: {
      name: service.id,
      labels: {
        'controller-tools.k8s.io': '1.0',
        serviceId: service.id
      }
    },
    spec: {
      name: service.name,
      id: service.id,
      bindable: service.bindable,
      description: service.description,
      metadata: service.metadata,
      tags: service.tags,
      dashboardClient: service.dashboard_client,
      planUpdateable: service.plan_updateable
    }
  };
  return serviceCRD;
}



function getPlanCrdFromConfig(plan, service) {
  assert.ok(plan.name, 'plan.name is required to generate plan crd');
  assert.ok(plan.id, 'plan.id is required to generate plan crd');
  assert.ok(plan.description, 'plan.description is required to generate plan crd');

  let planCRD = {
    apiVersion: 'osb.servicefabrik.io/v1alpha1',
    kind: 'SFPlan',
    metadata: {
      name: plan.id,
      labels: {
        'controller-tools.k8s.io': '1.0',
        serviceId: service.id,
        planId: plan.id
      }
    },
    spec: {
      name: plan.name,
      id: plan.id,
      serviceId: service.id,
      description: plan.description,
      free: plan.free ? true : service.free ? true : false,
      bindable: plan.bindable ? plan.bindable : service.bindable ? service.bindable : false,
      planUpdatable: plan.bindable ? true : false,
      templates: plan.templates ? plan.templates : [],
      metadata: plan.metadata,
      manager: plan.manager,
      context: plan.context
    }
  };
  return planCRD;
}

function pushServicePlanToApiServer() {
  return Promise.map(_.get(config, 'services', []), service => {
    const servicePromise = apiServerClient.createOrUpdateServicePlan(getServiceCrdFromConfig(service));
    return Promise.map(service.plans, plan => apiServerClient.createOrUpdateServicePlan(getPlanCrdFromConfig(plan, service))
      .then(() => servicePromise));
  });
}
function loadCatalogFromAPIServer() {
  return getAllServices()
    .tap(services => {
      config.services = services;
    })
    .then(services => {
      return Promise.all(Promise.each(services, service => {
        return getAllPlansForService(service.id)
          .then(plans => {
            service.plans = plans;
          });
      }));
    })
    .then(() => catalog.reload())
    .tap(() => logger.silly('Loaded Services in catalog Are ', catalog.services))
    .tap(() => logger.silly('Loaded Plans in catalog Are ', catalog.plans));
}

module.exports = {
  loadCatalogFromAPIServer,
  registerSFEventsCrd,
  waitWhileCRDsAreRegistered,
  pushServicePlanToApiServer,
  getServiceCrdFromConfig,
  getAllServices,
  getAllPlansForService
};
