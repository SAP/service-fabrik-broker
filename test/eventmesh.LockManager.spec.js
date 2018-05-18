'use strict';

const Promise = require('bluebird');
const sinon = require('sinon');
const manager = require('../eventmesh').lockManager;
const CONST = require('../eventmesh/constants');

const {
  Etcd3
} = require('etcd3');

describe('eventmesh', () => {
  describe('LockManager', () => {
    let sandbox, valueStub, stringStub, jsonStub, putStub, getStub, lockStub;
    before(() => {
      sandbox = sinon.sandbox.create();
      valueStub = sandbox.stub();
      stringStub = sandbox.stub();
      jsonStub = sandbox.stub();
      putStub = sandbox.stub(Etcd3.prototype, 'put', () => {
        return {
          value: (val) => Promise.resolve(valueStub(val))
        };
      });
      getStub = sandbox.stub(Etcd3.prototype, 'get', () => {
        return {
          json: () => Promise.resolve(jsonStub())
        };
      });
      lockStub = sandbox.stub(Etcd3.prototype, 'lock', () => {
        return {
          ttl: () => {
            return {
              acquire: () => Promise.resolve(stringStub())
            };
          },
          release: () => Promise.resolve(stringStub())
        };
      });
    })

    afterEach(function () {
      valueStub.reset();
      putStub.reset();
      getStub.reset();
      jsonStub.reset();
      stringStub.reset();
    });
    after(function () {
      sandbox.restore();
    });

    describe('#isWriteLocked', () => {
      it('should return false in case the resource has no lock', () => {
        return manager.isWriteLocked('fakeResource')
          .then(() => {
            /* jshint expr: true */
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
          });
      });
      it('should return true for a write lock.', () => {
        const writeLockResp = {
          'count': 1,
          'operationType': 'WRITE'
        };
        jsonStub.onCall(0).returns(writeLockResp);
        return manager.isWriteLocked('fakeResource')
          .then(result => {
            /* jshint expr: true */
            expect(result).to.eql(true);
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
          });
      });
      it('should return false for a read lock.', () => {
        const readLockResp = {
          'count': 1,
          'operationType': 'READ'
        };
        jsonStub.onCall(0).returns(readLockResp);
        return manager.isWriteLocked('fakeResource')
          .then(result => {
            expect(result).to.eql(false);
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
          });
      });
      it('should return false for a no lock.', () => {
        const noLockResp = {
          'count': 0,
          'operationType': ''
        };
        jsonStub.onCall(0).returns(noLockResp);
        return manager.isWriteLocked('fakeResource')
          .then(result => {
            expect(result).to.eql(false);
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
          });
      });
    });

    describe('#lock', () => {
      it('should succeed when lock details is undefined.', () => {
        const writeLockResp = {
          'count': 1,
          'operationType': 'WRITE'
        };
        return manager.lock('fakeResource', CONST.LOCK_TYPE.WRITE)
          .then(() => {
            expect(lockStub.getCall(0).calledWithExactly('fakeResource/lock')).to.be.true;
            expect(putStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(valueStub.getCall(0).calledWithExactly(JSON.stringify(writeLockResp))).to.be.true;
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(stringStub.called).to.be.true;
          });
      });
      it('should succeed if no ongoing lock is there.', () => {
        const noLockResp = {
          'count': 0,
          'operationType': ''
        };
        const writeLockResp = {
          'count': 1,
          'operationType': 'WRITE'
        };
        jsonStub.onCall(0).returns(noLockResp);
        return manager.lock('fakeResource', CONST.LOCK_TYPE.WRITE)
          .then(() => {
            expect(lockStub.getCall(0).calledWithExactly('fakeResource/lock')).to.be.true;
            expect(putStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(valueStub.getCall(0).calledWithExactly(JSON.stringify(writeLockResp))).to.be.true;
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(stringStub.called).to.be.true;
          });
      });
      it('should fail if an ongoing lock is there.', () => {
        const writeLockResp = {
          'count': 1,
          'operationType': 'WRITE'
        };
        jsonStub.onCall(0).returns(writeLockResp);
        return manager.lock('fakeResource', CONST.LOCK_TYPE.WRITE)
          .catch(e => {
            expect(e.message).to.eql('Could not acquire lock');
            expect(lockStub.getCall(0).calledWithExactly('fakeResource/lock')).to.be.true;
            expect(getStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(stringStub.called).to.be.true;
          });
      });
    });
    describe('#unlock', () => {
      it('should succeed.', () => {
        const noLockResp = {
          'count': 0,
          'operationType': ''
        };
        return manager.unlock('fakeResource')
          .then(() => {
            expect(putStub.getCall(0).calledWithExactly('fakeResource/lock/details')).to.be.true;
            expect(valueStub.getCall(0).calledWithExactly(JSON.stringify(noLockResp))).to.be.true;
            expect(stringStub.called).to.be.false;
          });
      });
    });
  });
});