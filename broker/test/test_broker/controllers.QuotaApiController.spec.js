'use strict';

const {
  getQuotaManagerInstance
} = require('@sf/quota');
const {
  CONST
} = require('@sf/common-utils');
const QuotaApiController = require('../../applications/quota-app/src/QuotaApiController');

class Response {
  constructor() {
    this.constructor.methods.forEach(method => {
      this[method] = sinon.spy(function () {
        return this;
      });
    });
  }
  reset() {
    this.constructor.methods.forEach(method => {
      this[method].resetHistory();
    });
  }
  static get methods() {
    return ['set', 'status', 'sendStatus', 'send', 'json', 'render', 'format'];
  }
}

describe('#getQuotaValidStatus', () => {
  const previous_plan_id = 'd616b00a-5949-4b1c-bc73-0d3c59f3954a';
  const organization_guid = 'b8cbbac8-6a20-42bc-b7db-47c205fccf9a';
  const subaccount_id = 'b319968c-0eba-43f2-959b-40f507c269fd';
  const notEntitledPlanId = 'bc158c9a-7934-401e-94ab-057082a5073e';
  const validQuotaPlanId = 'bc158c9a-7934-401e-94ab-057082a5073f';
  const invalidQuotaPlanId = 'd616b00a-5949-4b1c-bc73-0d3c59f3954a';
  const cfQuotaManager = getQuotaManagerInstance(CONST.PLATFORM.CF);
  const k8squotaManager = getQuotaManagerInstance(CONST.PLATFORM.K8S);
  let checkCFQuotaStub, checkK8SQuotaStub;
  const quotaApiController = new QuotaApiController();
  const res = new Response();
  beforeEach(function () {
    checkCFQuotaStub = sinon.stub(cfQuotaManager, 'checkQuota');
    checkK8SQuotaStub = sinon.stub(k8squotaManager, 'checkQuota');
    checkCFQuotaStub.withArgs(subaccount_id, organization_guid, notEntitledPlanId, previous_plan_id, 'PATCH').returns(Promise.resolve(CONST.QUOTA_API_RESPONSE_CODES.NOT_ENTITLED));
    checkCFQuotaStub.withArgs(subaccount_id, organization_guid, invalidQuotaPlanId, previous_plan_id, 'PATCH').returns(Promise.resolve(CONST.QUOTA_API_RESPONSE_CODES.INVALID_QUOTA));
    checkCFQuotaStub.withArgs(subaccount_id, organization_guid, validQuotaPlanId, previous_plan_id, 'PATCH').returns(Promise.resolve(CONST.QUOTA_API_RESPONSE_CODES.VALID_QUOTA));
    checkK8SQuotaStub.withArgs(subaccount_id, organization_guid, validQuotaPlanId, previous_plan_id, 'PATCH').returns(Promise.resolve(CONST.QUOTA_API_RESPONSE_CODES.VALID_QUOTA));
    checkK8SQuotaStub.withArgs(subaccount_id, organization_guid, validQuotaPlanId, previous_plan_id, 'PATCH', true).returns(Promise.resolve(CONST.QUOTA_API_RESPONSE_CODES.VALID_QUOTA));
  });
  afterEach(function () {
    res.reset();
    checkCFQuotaStub.restore();
    checkK8SQuotaStub.restore();
  });
  it('K8S deployment, subaccount based check, should return valid', async () => {
    const req = {
      params: {
        accountId: subaccount_id
      },
      query: {
        planId: validQuotaPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        useAPIServerForConsumedQuotaCheck: 'true',
        orgId: organization_guid
      }
    };
    process.env.POD_NAMESPACE = 'default';
    await quotaApiController.getQuotaValidStatus(req, res);
    delete process.env.POD_NAMESPACE;
    expect(checkK8SQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.OK);
    expect(res.send).to.have.been.calledWith({quotaValidStatus: 0});
  });
  it('K8S deployment, org based check, should return valid', async () => {
    const req = {
      params: {
        accountId: subaccount_id
      },
      query: {
        planId: validQuotaPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        useAPIServerForConsumedQuotaCheck: 'false',
        orgId: organization_guid
      }
    };
    process.env.POD_NAMESPACE = 'default';
    await quotaApiController.getQuotaValidStatus(req, res);
    delete process.env.POD_NAMESPACE;
    expect(checkK8SQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.OK);
    expect(res.send).to.have.been.calledWith({quotaValidStatus: 0});
  });
  it('Quota not entitled, return not entitled status', async () => {
    const req = {
      params: {
        accountId: subaccount_id
      },
      query: {
        planId: notEntitledPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        useAPIServerForConsumedQuotaCheck: 'false',
        orgId: organization_guid
      }
    };
    await quotaApiController.getQuotaValidStatus(req, res);
    expect(checkCFQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.OK);
    expect(res.send).to.have.been.calledWith({quotaValidStatus: 2});    
  });
  it('Quota invalid, return invalid status', async () => {
    const req = {
      params: {
        accountId: subaccount_id
      },
      query: {
        planId: invalidQuotaPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        useAPIServerForConsumedQuotaCheck: 'false',
        orgId: organization_guid
      }
    };
    await quotaApiController.getQuotaValidStatus(req, res);
    expect(checkCFQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.OK);
    expect(res.send).to.have.been.calledWith({quotaValidStatus: 1});     
  });
  it('Quota valid, should return valid status', async () => {
    const req = {
      params: {
        accountId: subaccount_id
      },
      query: {
        planId: validQuotaPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        useAPIServerForConsumedQuotaCheck: 'false',
        orgId: organization_guid
      }
    };
    await quotaApiController.getQuotaValidStatus(req, res);
    expect(checkCFQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.OK);
    expect(res.send).to.have.been.calledWith({quotaValidStatus: 0});      
  });

  it('Quota funtion throws error, should return error', async () => {
    const err = 'Error in calculating quota';
    const req = {
      params: {
        accountId: organization_guid
      },
      query: {
        planId: validQuotaPlanId,
        previousPlanId: previous_plan_id,
        reqMethod: 'PATCH',
        isSubaccountFlag: 'false'
      }
    };
    checkCFQuotaStub.reset();
    checkCFQuotaStub.returns(Promise.reject(err));
    await quotaApiController.getQuotaValidStatus(req, res);
    expect(checkCFQuotaStub).to.have.been.called;
    expect(res.status).to.have.been.calledOnce;
    expect(res.status).to.have.been.calledWith(CONST.HTTP_STATUS_CODE.INTERNAL_SERVER_ERROR);
    expect(res.send).to.have.been.calledWith({ error: err });    
  });
});