'use strict';

const lib = require('../lib');
const api = lib.controllers.serviceBrokerApi;
const errors = lib.errors;
const cloudController = lib.cf.cloudController;
const PreconditionFailed = errors.PreconditionFailed;
const ContinueWithNext = errors.ContinueWithNext;

describe('fabrik', function () {
  describe('ServiceBrokerApi', function () {
    describe('#apiVersion', function () {

      const req = {
        headers: {
          'x-broker-api-version': 2.7
        }
      };
      const res = {};

      function expectToThrow(clazz) {
        return new Error(`Expected error '${clazz.name}' has not been thrown`);
      }

      after(function () {
        cloudController.apiVersion = undefined;
      });

      it('should abort with a PreconditionFailed error when version is 2.7', function () {
        req.headers['x-broker-api-version'] = '2.7';
        return api
          .apiVersion(req, res)
          .throw(expectToThrow(PreconditionFailed))
          .catch(err => expect(err).to.be.instanceof(PreconditionFailed));
      });

      it('should call the next handler when version is 2.8', function () {
        cloudController.apiVersion = '2.55.0';
        req.headers['x-broker-api-version'] = '2.8';
        return api
          .apiVersion(req, res)
          .throw(expectToThrow(PreconditionFailed))
          .catch(err => expect(err).to.be.instanceof(PreconditionFailed));
      });

      it('should call the next handler when version is 2.8', function () {
        cloudController.apiVersion = '2.57.0';
        req.headers['x-broker-api-version'] = '2.8';
        return api
          .apiVersion(req, res)
          .throw(expectToThrow(ContinueWithNext))
          .catch(err => expect(err).to.be.instanceof(ContinueWithNext));
      });

      it('should call the next handler when version is 2.9', function () {
        req.headers['x-broker-api-version'] = '2.9';
        return api
          .apiVersion(req, res)
          .throw(expectToThrow(ContinueWithNext))
          .catch(err => expect(err).to.be.instanceof(ContinueWithNext));
      });

    });
  });
});