'use strict';
const EventMeshServer = require('../../eventmesh/EventMeshServer');
const CONST = require('../../common/constants');
const errors = require('../../common/errors');
const NotImplementedBySubclass = errors.NotImplementedBySubclass;

describe('eventmesh', () => {
  describe('EventMeshServer', () => {
    let eventmesh = new EventMeshServer();

    describe('#checkValidState', () => {
      it('should throw error if state is invalid', () => {
        return eventmesh.checkValidState()
          .catch(e => expect(e.message).to.eql('Could not find state undefined'));
      });
      it('should return if state is valid', () => {
        return eventmesh.checkValidState(CONST.APISERVER.RESOURCE_STATE.IN_QUEUE)
          .catch(() => {
            throw new Error('No exception expected');
          });
      });
    });

    describe('#getServiceFolderName', () => {
      it('should return the key name for services', () => {
        expect(eventmesh.getServiceFolderName('foo', 'bar')).to.eql('services/foo/bar');
      });
    });

    describe('#getResourceFolderName', () => {
      it('should return the key name for resources', () => {
        expect(eventmesh.getResourceFolderName('foo', 'bar')).to.eql('deployments/foo/bar');
      });
    });

    describe('#getAnnotationFolderName', () => {
      it('should return the key name for annotation', () => {
        const opts = {
          resourceId: 'fakeResourceId',
          annotationName: 'fakeAnnotationName',
          annotationType: 'fakeAnnotationType',
          annotationId: 'fakeAnnotationId',
        };
        expect(eventmesh.getAnnotationFolderName(opts))
          .to.eql(`${opts.annotationName}/${opts.annotationType}/${opts.resourceId}/${opts.annotationId}`);
      });
    });

    describe('#registerService', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.registerService).to.throw(NotImplementedBySubclass);
        expect(eventmesh.registerService).to.throw('registerService');
      });
    });

    describe('#getServiceAttributes', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getServiceAttributes).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getServiceAttributes).to.throw('getServiceAttributes');
      });
    });

    describe('#getServicePlans', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getServicePlans).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getServicePlans).to.throw('getServicePlans');
      });
    });

    describe('#createDeploymentResource', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.createDeploymentResource).to.throw(NotImplementedBySubclass);
        expect(eventmesh.createDeploymentResource).to.throw('createDeploymentResource');
      });
    });

    describe('#updateResourceState', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.updateResourceState).to.throw(NotImplementedBySubclass);
        expect(eventmesh.updateResourceState).to.throw('updateResourceState');
      });
    });

    describe('#updateResourceKey', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.updateResourceKey).to.throw(NotImplementedBySubclass);
        expect(eventmesh.updateResourceKey).to.throw('updateResourceKey');
      });
    });

    describe('#getResourceKeyValue', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getResourceKeyValue).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getResourceKeyValue).to.throw('getResourceKeyValue');
      });
    });

    describe('#getResourceState', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getResourceState).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getResourceState).to.throw('getResourceState');
      });
    });

    describe('#registerWatcher', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.registerWatcher).to.throw(NotImplementedBySubclass);
        expect(eventmesh.registerWatcher).to.throw('registerWatcher');
      });
    });

    describe('#createOperationResource', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.createOperationResource).to.throw(NotImplementedBySubclass);
        expect(eventmesh.createOperationResource).to.throw('createOperationResource');
      });
    });

    describe('#updateAnnotationState', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.updateAnnotationState).to.throw(NotImplementedBySubclass);
        expect(eventmesh.updateAnnotationState).to.throw('updateAnnotationState');
      });
    });

    describe('#updateAnnotationKey', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.updateAnnotationKey).to.throw(NotImplementedBySubclass);
        expect(eventmesh.updateAnnotationKey).to.throw('updateAnnotationKey');
      });
    });

    describe('#getAnnotationKeyValue', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getAnnotationKeyValue).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getAnnotationKeyValue).to.throw('getAnnotationKeyValue');
      });
    });

    describe('#getAnnotationState', () => {
      it('should thow error if not implemented by subclass', () => {
        expect(eventmesh.getAnnotationState).to.throw(NotImplementedBySubclass);
        expect(eventmesh.getAnnotationState).to.throw('getAnnotationState');
      });
    });

  });
});