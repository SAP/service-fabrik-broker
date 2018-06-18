'use strict';

const _ = require('lodash');
const Promise = require('bluebird');
const assert = require('assert');
const config = require('../common/config');
const logger = require('../common/logger');
const CONST = require('../common/constants');
const EventMeshServer = require('./EventMeshServer');
const kc = require('kubernetes-client');
const JSONStream = require('json-stream');


const apiserver = new kc.Client({
  config: {
    url: `https://${config.internal.ip}:9443`,
    insecureSkipTlsVerify: true
  },
  version: '1.9'
});

class ApiServerEventMesh extends EventMeshServer {
  registerWatcher(resourceName, resourceType, callback) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => {
        const stream = apiserver
          .apis[`${resourceName}.servicefabrik.io`]
          .v1alpha1.watch.namespaces('default')[resourceType].getStream();
        const jsonStream = new JSONStream();
        stream.pipe(jsonStream);
        jsonStream.on('data', callback);
      })
      .catch(e => logger.error('Caucht error while registering', e));
  }
  createLockResource(name, type, body) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${name}.servicefabrik.io`].v1alpha1.namespaces('default')[type].post({
        body: body
      }));
  }
  deleteLockResource(name, type, resourceName) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${name}.servicefabrik.io`].v1alpha1.namespaces('default')[type](resourceName).delete());
  }
  updateLockResource(name, type, resourceName, delta) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${name}.servicefabrik.io`].v1alpha1.namespaces('default')[type](resourceName).patch({
        body: delta
      }));
  }
  getLockResourceOptions(name, type, resourceName) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${name}.servicefabrik.io`].v1alpha1.namespaces('default')[type](resourceName).get())
      .then(resource => {
        return resource.body.spec.options;
      })
  }
  getResource(name, type, resourceName) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${name}.servicefabrik.io`].v1alpha1.namespaces('default')[type](resourceName).get());
  }

  createResource(resourceType, resourceId, val) {
    logger.debug(`Creating Resource ${resourceType}/${resourceId}`);

    const initialResource = {
      apiVersion: "deployment.servicefabrik.io/v1alpha1",
      metadata: {
        name: resourceId,
        "labels": {
          instance_guid: `${resourceId}`,
        },
      },
      spec: {
        "options": JSON.stringify(val)
      },
    };

    const statusJson = {
      status: {
        state: CONST.APISERVER.STATE.IN_QUEUE,
        lastOperation: "created",
        response: JSON.stringify({})
      }
    }

    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis['deployment.servicefabrik.io']
        .v1alpha1.namespaces('default').directors.post({
          body: initialResource
        }))
      .then(() => apiserver.apis['deployment.servicefabrik.io']
        .v1alpha1.namespaces('default').directors(resourceId).status.patch({
          body: statusJson
        }))
  }

  updateResourceState(resourceType, resourceId, stateValue) {
    const patchedResource = {
      "status": {
        "state": stateValue
      }
    }
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`deployment.servicefabrik.io`]
        .v1alpha1.namespaces('default')[resourceType](resourceId)
        .status.patch({
          body: patchedResource
        }))
  }

  getResourceState(resourceType, resourceId) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`deployment.servicefabrik.io`]
        .v1alpha1.namespaces('default')[resourceType](resourceId)
        .get())
      .then(json => json.body.status.state);
  }

  /**
   *
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * @params opts.val
   */
  annotateResource(opts) {
    logger.info('Creating resource with options:', opts.val)
    const initialResource = {
      "apiVersion": `${opts.annotationName}.servicefabrik.io/v1alpha1`,
      metadata: {
        "name": `${opts.annotationId}`,
        "labels": {
          instance_guid: `${opts.resourceId}`,
        },
      },
      spec: {
        "options": JSON.stringify(opts.val)
      },
    };
    const statusJson = {
      status: {
        state: CONST.APISERVER.STATE.IN_QUEUE,
        lastOperation: "",
        response: ""
      }
    }
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver.apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType].post({
          body: initialResource
        }))
      .then(() => apiserver.apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId).status.patch({
          body: statusJson
        }))
  }
  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * @params opts.value
   */
  updateAnnotationResult(opts) {
    const patchedResource = {
      "status": {
        "response": JSON.stringify(opts.value),
      }
    }
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId)
        .status.patch({
          body: patchedResource
        }))
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * @params opts.stateValue
   */
  updateAnnotationState(opts) {
    assert.ok(opts.resourceId, `Property 'resourceId' is required to update annotation state`);
    assert.ok(opts.annotationName, `Property 'annotationName' is required to update annotation state`);
    assert.ok(opts.annotationType, `Property 'annotationType' is required to update annotation state`);
    assert.ok(opts.annotationId, `Property 'annotationId' is required to update annotation state`);
    assert.ok(opts.stateValue, `Property 'stateValue' is required to update annotation state`);
    const patchedResource = {
      "status": {
        "state": opts.stateValue
      }
    }
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId)
        .status.patch({
          body: patchedResource
        }))
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.value
   */
  updateLastAnnotation(opts) {
    const patchedResource = {}
    patchedResource["metadata"] = {}
    patchedResource.metadata["labels"] = {}
    patchedResource.metadata.labels[`last_${opts.annotationName}_${opts.annotationType}`] = opts.value
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis['deployment.servicefabrik.io']
        .v1alpha1.namespaces('default')["directors"](opts.resourceId)
        .patch({
          body: patchedResource
        }))
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   */
  getLastAnnotation(opts) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`deployment.servicefabrik.io`]
        .v1alpha1.namespaces('default')["directors"](opts.resourceId)
        .get())
      .then(json => json.body.metadata.labels[`last_${opts.annotationName}_${opts.annotationType}`]);
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * returns string
   */
  getAnnotationOptions(opts) {
    assert.ok(opts.resourceId, `Property 'resourceId' is required to get annotation state`);
    assert.ok(opts.annotationName, `Property 'annotationName' is required to get annotation state`);
    assert.ok(opts.annotationType, `Property 'annotationType' is required to get annotation state`);
    assert.ok(opts.annotationId, `Property 'annotationId' is required to get annotation state`);
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId)
        .get())
      .then(json => json.body.spec.options)
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * returns string
   */
  getAnnotationState(opts) {
    assert.ok(opts.resourceId, `Property 'resourceId' is required to get annotation state`);
    assert.ok(opts.annotationName, `Property 'annotationName' is required to get annotation state`);
    assert.ok(opts.annotationType, `Property 'annotationType' is required to get annotation state`);
    assert.ok(opts.annotationId, `Property 'annotationId' is required to get annotation state`);
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId)
        .get())
      .then(json => json.body.status.state)
  }

  /**
   * @params opts.resourceId
   * @params opts.annotationName
   * @params opts.annotationType
   * @params opts.annotationId
   * returns string
   */
  getAnnotationResult(opts) {
    return Promise.try(() => apiserver.loadSpec())
      .then(() => apiserver
        .apis[`${opts.annotationName}.servicefabrik.io`]
        .v1alpha1.namespaces('default')[opts.annotationType](opts.annotationId)
        .get())
      .then(json => json.body.status.response)
  }

}

module.exports = ApiServerEventMesh;