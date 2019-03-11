'use strict';
const _ = require('lodash');
const Promise = require('bluebird');
const logger = require('../../common/logger');
// const errors = require('../../common/errors');
// const utils = require('../../common/utils');
const AliClient = require('../../data-access-layer/iaas').AliClient;
const AliStorage = require('ali-oss');
// const NotFound = errors.NotFound;
// const Forbidden = errors.Forbidden;
// const UnprocessableEntity = errors.UnprocessableEntity;

const CONNECTION_WAIT_SIMULATED_DELAY = 5;
const config = {
  backup: {
    retention_period_in_days: 14,
    max_num_on_demand_backup: 2,
    status_check_every: 120000, // (ms) Check the status of backup once every 2 mins
    backup_restore_status_poller_timeout: 86400000, // (ms) Deployment backup/restore must finish within this timeout time (24 hrs)
    backup_restore_status_check_every: 120000, // (ms) Check the status of deployment backup/restore once every 2 mins
    abort_time_out: 300000, //(ms) Timeout time for abort of backup to complete
    provider: {
      keyId: 'key-id',
      container: 'sample-container',
      endpoint: 'https://sample-endpoint',
      name: 'ali',
      region: 'region-name',
      key: 'secret-key'
    }
  }
};
const settings = config.backup.provider;
const bucketMetadataResponse = {
  buckets: [{
    name: 'sample-container',
    id: 'sample-container',
    timeCreated: '2017-12-24T10:23:50.348Z',
    updated: '2017-12-24T10:23:50.348Z',
    location: 'region-name',
  }]
};
const listFilesResponse = {
  objects:
    [{
      name: 'blob1.txt',
      lastModified: '2018-03-08T14:15:49.655Z'
    },
    {
      name: 'blob2.txt',
      lastModified: '2018-03-08T14:15:49.655Z'
    }]
};
const deleteFileSuccessResponse = {
  undefined
};

const validBlobName = 'blob1.txt';
const jsonContent = {
  content: '{"data": "This is a sample content"}'
};

const bucketStub = function () {
  return Promise.resolve(bucketMetadataResponse);
};
const listFilesStub = {
  list: () => {
    return Promise.resolve(listFilesResponse);
  },
  get: () => {
    return Promise.resolve(jsonContent);
  },
  put: () => {
    return Promise.resolve(jsonContent);
  },
  delete: (file) => {
    if (file === validBlobName) {
      return Promise.resolve(deleteFileSuccessResponse);
    }
  }
};
describe('iaas', function () {
  describe('AliClient', function () {
    describe('#AliStorage', function () {
      it('should form an object with correct credentials', function () {
        const responseAliStorageObject = AliClient.createStorageClient(settings);

        expect(responseAliStorageObject.options.accessKeyId).to.equal(settings.keyId);
        expect(responseAliStorageObject.options.accessKeySecret).to.equal(settings.key);
        expect(responseAliStorageObject.options.region).to.equal(settings.region);
        expect(responseAliStorageObject.options.endpoint.href).to.equal(settings.endpoint + '/');
        expect(responseAliStorageObject.options.endpoint.hostname).to.equal(_.split(settings.endpoint, '//')[1]);
        expect(responseAliStorageObject.options.endpoint.protocol).to.equal(_.split(settings.endpoint, '//')[0]);
      });
    });

    describe('#BucketOperations', function () {
      let sandbox, client;
      before(function () {
        sandbox = sinon.createSandbox();
        client = new AliClient(settings);
        sandbox.stub(AliStorage.prototype, 'listBuckets').withArgs({ prefix: settings.container }).callsFake(bucketStub);
        sandbox.stub(AliStorage.prototype, 'useBucket').withArgs(settings.container).returns(listFilesStub);
      });
      after(function () {
        sandbox.restore();
      });

      it('container properties should be retrived successfully', function () {
        return client.getContainer()
          .then(result => {
            expect(result[0].name).to.equal(settings.container);
            expect(result[0].id).to.equal(settings.container);
            expect(result[0].location).to.equal(settings.region);
          })
          .catch(err => {
            logger.error(err);
            console.log(err);
            throw new Error('expected container properties to be retrived successfully');
          });
      });
      it('list of files/blobs should be returned', function () {
        const options = {
          prefix: 'blob'
        };
        const expectedResponses = [{
          name: 'blob1.txt',
          lastModified: '2018-03-08T14:15:49.655Z'
        },
        {
          name: 'blob2.txt',
          lastModified: '2018-03-08T14:15:49.655Z'
        }
        ];
        return client.list(options)
          .then(results => {
            expect(results).to.be.an('array');
            expect(results).to.have.lengthOf(2);
            expect(results[0].name).to.equal(expectedResponses[0].name);
            expect(results[0].lastModified).to.equal(expectedResponses[0].lastModified);
            expect(results[1].name).to.equal(expectedResponses[1].name);
            expect(results[1].lastModified).to.equal(expectedResponses[1].lastModified);
          })
          .catch(err => {
            logger.error(err);
            throw new Error('expected list of files/blobs to be returned successfully');
          });
      });
      it('file/blob deletion should be successful', function () {
        return client.remove(validBlobName)
          .then(result => expect(result).to.be.undefined)
          .catch(err => {
            logger.error(err);
            throw new Error('expected file/blob deletion to be successful');
          });
      });
      it('file/blob download should be successful', function () {
        return client.downloadJson(validBlobName)
          .then(response => expect(response).to.eql(JSON.parse(jsonContent.content)))
          .catch(() => {
            throw new Error('expected download to be successful');
          });
      });
      it('file/blob upload should be successful', function () {
        return client.uploadJson(validBlobName, jsonContent)
          .then(response => expect(response.content).to.eql(jsonContent.content));
      });
    });
  });
});
