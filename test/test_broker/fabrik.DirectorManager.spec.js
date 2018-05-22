'use strict';

const _ = require('lodash');
const yaml = require('js-yaml');
const lib = require('../../broker/lib');
const catalog = lib.models.catalog;
const proxyquire = require('proxyquire');
const Promise = require('bluebird');
const errors = require('../../broker/lib/errors');
const CONST = require('../../broker/lib/constants');
const ServiceInstanceAlreadyExists = errors.ServiceInstanceAlreadyExists;

var used_guid = '4a6e7c34-d97c-4fc0-95e6-7a3bc8030be9';
var free_guid = '87599704-adc9-1acd-0be9-795e6a3bc803';
var boshStub = {
  NetworkSegmentIndex: {
    adjust: function (num) {
      return num;
    },
    findFreeIndex: function () {
      return 2;
    }
  },
  director: {
    getDeploymentNames: function () {
      return Promise.resolve([`service-fabrik-0021-${used_guid}`]);
    },
    getDeploymentNameForInstanceId: function () {
      return Promise.resolve([`service-fabrik-0021-${used_guid}`]);
    }
  }
};

var DirectorManager = proxyquire('../../broker/lib/fabrik/DirectorManager', {
  '../bosh': boshStub,
});

describe('fabrik', function () {
  describe('DirectorManager', function () {
    const plan_id = 'bc158c9a-7934-401e-94ab-057082a5073f';
    const xsmall_plan_id = plan_id;
    const small_plan_id = 'bc158c9a-7934-401e-94ab-057082a5073e';
    let return_value;
    let manager;

    before(function () {
      manager = new DirectorManager(catalog.getPlan(plan_id));
    });
    afterEach(function () {
      mocks.reset();
    });
    describe('#getDeploymentName', function () {
      it('should append guid and network segment index to deployment name', function () {
        expect(manager.plan.id).to.eql(plan_id);
        expect(manager.getDeploymentName(used_guid, '90')).to.eql(`service-fabrik-90-${used_guid}`);
        manager.aquireNetworkSegmentIndex(used_guid)
          .catch(err => expect(err).to.be.instanceof(ServiceInstanceAlreadyExists));
        manager.aquireNetworkSegmentIndex(free_guid).then(index => expect(index).to.eql(2));
      });
    });
    describe('#findNetworkSegmentIndex', function () {
      it('should append guid and network segment index to deployment name', function () {
        manager.findNetworkSegmentIndex(used_guid).then(res => expect(res).to.eql(21));
      });
    });
    describe('#isRestorePossible', function () {
      it('should return false when plan not in restore_predecessors', function () {
        // restore not possible from small to xsmall
        manager = new DirectorManager(catalog.getPlan(xsmall_plan_id));
        manager.update_predecessors = [];
        return_value = expect(manager.isRestorePossible(small_plan_id)).to.be.false;
      });
      it('should return true when plan not in restore_predecessors', function () {
        // restore possible from xsmall to small
        manager = new DirectorManager(catalog.getPlan(small_plan_id));
        manager.update_predecessors = [xsmall_plan_id];
        return_value = expect(manager.isRestorePossible(xsmall_plan_id)).to.be.true;
      });
    });
    describe('#restorePredecessors', function () {
      it('should return update_predecessors if restore_predecessors is not defined', function () {
        manager = new DirectorManager(catalog.getPlan(small_plan_id));
        manager.update_predecessors = [xsmall_plan_id];
        expect(manager.restorePredecessors).to.eql(manager.update_predecessors);
      });
    });

    describe('#executeActions', function () {
      before(function () {
        return mocks.setup([]);
      });

      afterEach(function () {
        mocks.reset();
      });
      const rabbit_plan_id = 'b715f834-2048-11e7-a560-080027afc1e6';
      const context = {
        deployment_name: 'my-deployment'
      };
      it('should return empty response if no actions are defined', function () {
        manager = new DirectorManager(catalog.getPlan(rabbit_plan_id));
        return manager.executeActions(CONST.SERVICE_LIFE_CYCLE.PRE_CREATE, context)
          .then(actionResponse => {
            expect(actionResponse).to.eql({});
          });
      });
      it('should return empty response if actions are not provided', function () {
        manager = new DirectorManager(catalog.getPlan(small_plan_id));
        let temp_actions = manager.service.actions;
        manager.service.actions = '';
        return manager.executeActions(CONST.SERVICE_LIFE_CYCLE.PRE_CREATE, context)
          .then(actionResponse => {
            manager.service.actions = temp_actions;
            expect(actionResponse).to.eql({});
          });
      });
      it('should return correct action response', function () {
        const expectedRequestBody = {
          phase: 'PreCreate',
          actions: ['Blueprint', 'ReserveIps'],
          context: {
            deployment_name: 'my-deployment'
          }
        };
        mocks.deploymentHookClient.executeDeploymentActions(200, expectedRequestBody);
        manager = new DirectorManager(catalog.getPlan(xsmall_plan_id));
        return manager.executeActions(CONST.SERVICE_LIFE_CYCLE.PRE_CREATE, context)
          .then(actionResponse => {
            expect(actionResponse).to.eql({});
            mocks.verify();
          });
      });
    });
    describe('#configureAddOns', function () {
      it('should update manifest with addons', function () {
        const plan = _.cloneDeep(catalog.getPlan(plan_id));
        const directorManager = new DirectorManager(plan);
        const updatedTemplate = directorManager.template + '\n' +
          'addons: \n' +
          '  - name: service-addon \n' +
          '    jobs: \n' +
          '    - name: service-addon \n' +
          '      release: service-release';
        directorManager.plan.manager.settings.template = Buffer.from(updatedTemplate).toString('base64');
        expect(directorManager.plan.id).to.eql(plan_id);
        expect(directorManager.getDeploymentName(used_guid, '90')).to.eql(`service-fabrik-90-${used_guid}`);
        const manifest = yaml.safeLoad(directorManager.generateManifest(`service-fabrik-90-${used_guid}`, {}));
        expect(manifest.addons.length).to.equal(2);
        expect(manifest.releases.length).to.equal(2);
      });
      it('should not update manifest with addons with parameter skip_addons set to true', function () {
        const directorManager = new DirectorManager(catalog.getPlan(plan_id));
        expect(directorManager.plan.id).to.eql(plan_id);
        expect(directorManager.getDeploymentName(used_guid, '90')).to.eql(`service-fabrik-90-${used_guid}`);
        const manifest = yaml.safeLoad(directorManager.generateManifest(`service-fabrik-90-${used_guid}`, {
          skip_addons: true
        }));
        expect(manifest.addons).to.equal(undefined);
        expect(manifest.releases.length).to.equal(1);
      });
    });
  });
});