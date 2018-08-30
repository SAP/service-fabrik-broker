'use strict';
// TODO abstract to a base poller

const _ = require('lodash');
const eventmesh = require('../../data-access-layer/eventmesh');
const CONST = require('../../common/constants');
const logger = require('../../common/logger');
const catalog = require('../../common/models').catalog;
const config = require('../../common/config');
const RestoreService = require('./');
const utils = require('../../common/utils');
const EventLogInterceptor = require('../../common/EventLogInterceptor');

class RestoreTaskPoller {

  static start() {
    function poller(object, interval) {
      // TODO handle HA scenario
      const response = JSON.parse(_.get(object.status, 'response'));
      const changedOptions = JSON.parse(object.spec.options);
      logger.info('Getting restore status with the following options and response:', changedOptions, response);
      const plan = catalog.getPlan(changedOptions.plan_id);
      const restore_opts = {
        context: changedOptions.context,
        agent_ip: response.agent_ip,
        instance_guid: changedOptions.instance_guid,
        restore_guid: changedOptions.restore_guid
      };
      return RestoreService.createService(plan)
        .then(restoreService => restoreService
          .getRestoreOperationState(restore_opts)
          .catch(e => logger.error('Caught error in poller', e))
          .then(operationStatusResponse => {
            logger.info(`Got restore operation response for guid ${changedOptions.restore_guid}`, operationStatusResponse);
            if (_.includes([CONST.APISERVER.RESOURCE_STATE.SUCCEEDED, CONST.APISERVER.RESOURCE_STATE.FAILED], operationStatusResponse.state)) {
              // cancel the poller and clear the array
              clearInterval(interval);
              RestoreTaskPoller.pollers[object.metadata.name] = false;
            }
            return Promise
              .try(() => {
                if (utils.isServiceFabrikOperationFinished(operationStatusResponse.state)) {
                  return this._logEvent(restore_opts, CONST.OPERATION_TYPE.RESTORE, operationStatusResponse);
                }
              })
              .then(() => eventmesh.apiServerClient.patchResource({
                resourceGroup: CONST.APISERVER.RESOURCE_GROUPS.RESTORE,
                resourceType: CONST.APISERVER.RESOURCE_TYPES.DEFAULT_RESTORE,
                resourceId: changedOptions.restore_guid,
                status: {
                  'state': operationStatusResponse.state,
                  response: operationStatusResponse
                }
              }));
          })
        );
    }

    function startPoller(event) {
      logger.debug('Received Restore Event: ', event);
      if ((event.type === CONST.API_SERVER.WATCH_EVENT.ADDED || event.type === CONST.API_SERVER.WATCH_EVENT.MODIFIED) && !RestoreTaskPoller.pollers[event.object.metadata.name]) {
        logger.info(`Starting poller for restore: ${event.object.metadata.name} with interval ${CONST.RESTORE_RESOURCE_POLLER_INTERVAL}`);
        const interval = setInterval(() => poller(event.object, interval), CONST.RESTORE_RESOURCE_POLLER_INTERVAL);
        RestoreTaskPoller.pollers[event.object.metadata.name] = true;
      }
    }
    const queryString = `state in (${CONST.APISERVER.RESOURCE_STATE.IN_PROGRESS})`;
    return eventmesh.apiServerClient.registerWatcher(
        CONST.APISERVER.RESOURCE_GROUPS.RESTORE,
        CONST.APISERVER.RESOURCE_TYPES.DEFAULT_RESTORE,
        startPoller, queryString)
      .then(stream => {
        logger.debug(`Successfully set watcher on restore resources`);
        return setTimeout(() => {
          try {
            logger.debug(`Refreshing stream after ${CONST.APISERVER.WATCHER_REFRESH_INTERVAL}`);
            stream.abort();
            return this.start();
          } catch (err) {
            logger.error('Error caught in the set timout callback for resource poller');
            throw err;
          }
        }, CONST.APISERVER.WATCHER_REFRESH_INTERVAL);
      })
      .catch(e => {
        logger.error('Failed registering watcher on restore resources with error:', e);
        throw e;
      });
  }

  static _logEvent(instanceInfo, operation, operationStatusResponse) {
    const eventLogger = EventLogInterceptor.getInstance(config.external.event_type, 'external');
    const check_res_body = true;
    const resp = {
      statusCode: 200,
      body: operationStatusResponse
    };
    if (CONST.URL[operation]) {
      return eventLogger.publishAndAuditLogEvent(CONST.URL[operation], CONST.HTTP_METHOD.POST, instanceInfo, resp, check_res_body);
    }
  }
}


RestoreTaskPoller.pollers = [];
module.exports = RestoreTaskPoller;