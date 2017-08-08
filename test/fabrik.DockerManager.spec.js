'use strict';

const lib = require('../lib');
const portRegistry = lib.docker.portRegistry;
const catalog = lib.models.catalog;
const DockerManager = lib.fabrik.DockerManager;

describe('fabrik', function () {
  describe('DockerManager', function () {
    /* jshint expr:true */

    const plan_id = '466c5078-df6e-427d-8fb2-c76af50c0f56';
    let manager;
    let sampleStub;
    let willBeExhaustedSoonSpy;
    let ports = [];

    function createManager(plan_id) {
      return new DockerManager(catalog.getPlan(plan_id));
    }

    before(function () {
      manager = createManager(plan_id);
      sampleStub = sinon.stub(portRegistry, 'sample', () => ports.shift());
      willBeExhaustedSoonSpy = sinon.spy(portRegistry, 'willBeExhaustedSoon');
    });

    beforeEach(function () {
      ports = [32768, 32769];
    });

    after(function () {
      portRegistry.willBeExhaustedSoon.restore();
      portRegistry.sample.restore();
    });

    describe('#createPortBindings', function () {
      it('should return port bindings', function () {
        const exposedPorts = {
          '314/tcp': {},
          '2718/tcp': {}
        };
        return manager
          .createPortBindings(exposedPorts)
          .then(portBindings => {
            expect(willBeExhaustedSoonSpy).to.be.calledTwice;
            expect(sampleStub).to.be.calledTwice.and.calledWith('tcp');
            expect(portBindings).to.eql({
              '314/tcp': [{
                HostPort: '32768'
              }],
              '2718/tcp': [{
                HostPort: '32769'
              }]
            });
          });
      });
    });
  });
});