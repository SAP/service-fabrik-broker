'use strict';

const lib = require('../../broker/lib');
const DirectorManager = require('../../broker/lib/fabrik').DirectorManager;
const utils = require('../../common/utils');

describe('utils', function () {
  describe('#deploymentNameRegExp', function () {
    let test_subnet = 'test-subnet';
    let deployment_name = `${DirectorManager.prefix}_${test_subnet}-1234-5432abcd-1098-abcd-7654-3210abcd9876`;

    it('should match network index', function () {
      expect(utils.deploymentNameRegExp(test_subnet).exec(deployment_name)[2]).to.eql('1234');
    });
    it('should match guid', function () {
      expect(utils.deploymentNameRegExp(test_subnet).exec(deployment_name)[3]).to.eql('5432abcd-1098-abcd-7654-3210abcd9876');
    });

    it('should match name and subnet', function () {
      expect(utils.deploymentNameRegExp(test_subnet).exec(deployment_name)[1]).to.eql('service-fabrik_test-subnet');
      // removesubnet 
      deployment_name = `${DirectorManager.prefix}-1234-5432abcd-1098-abcd-7654-3210abcd9876`;
      expect(utils.deploymentNameRegExp().exec(deployment_name)[1]).to.eql('service-fabrik');
      expect(utils.deploymentNameRegExp('').exec(deployment_name)[1]).to.eql('service-fabrik');
    });
  });

  describe('#taskIdRegExp', function () {
    it('should match name and taskId', function () {
      let prefixedTaskId = `${DirectorManager.prefix}-1234-5432abcd-1098-abcd-7654-3210abcd9876_12345`;
      expect(utils.taskIdRegExp().exec(prefixedTaskId)[1]).to.eql(`${DirectorManager.prefix}-1234-5432abcd-1098-abcd-7654-3210abcd9876`);
      expect(utils.taskIdRegExp().exec(prefixedTaskId)[2]).to.eql('12345');
    });
  });

  describe('#Random', function () {
    let randomIntStub;
    before(function () {
      randomIntStub = sinon.stub(utils, 'getRandomInt', () => 1);
    });
    after(function () {
      randomIntStub.restore();
    });
    it('should return a random cron expression for once every 15 days', function () {
      const AssertionError = require('assert').AssertionError;
      expect(utils.getRandomCronForOnceEveryXDays.bind(utils, 29)).to.throw(AssertionError);
      //bind returns a ref to function which is executed and checked for if it threw exception.
      expect(utils.getRandomCronForOnceEveryXDays(2)).to.eql('1 1 1,3,5,7,9,11,13,15,17,19,21,23,25,27,29 * *');
      expect(utils.getRandomCronForOnceEveryXDays(7)).to.eql('1 1 1,8,15,22 * *');
      expect(utils.getRandomCronForOnceEveryXDays(15)).to.eql('1 1 1,16 * *');
    });
    it('should return a random cron expression for every x hours for the day', function () {
      const AssertionError = require('assert').AssertionError;
      expect(utils.getRandomCronForEveryDayAtXHoursInterval.bind(utils, 29)).to.throw(AssertionError);
      //bind returns a ref to function which is executed and checked for if it threw exception.
      expect(utils.getRandomCronForEveryDayAtXHoursInterval(8)).to.eql('1 1,9,17 * * *');
      expect(utils.getRandomCronForEveryDayAtXHoursInterval(7)).to.eql('1 1,8,15,22 * * *');
      expect(utils.getRandomCronForEveryDayAtXHoursInterval(3)).to.eql('1 1,4,7,10,13,16,19,22 * * *');
    });
  });

  describe('#isServiceFabrikOperation', function () {
    /* jshint expr:true */
    const service_id = '24731fb8-7b84-4f57-914f-c3d55d793dd4';
    const plan_id_update = 'd616b00a-5949-4b1c-bc73-0d3c59f3954a';
    var queryParams = {
      service_id: service_id,
      plan_id: plan_id_update,
      previous_values: {
        service_id: service_id
      }
    };
    it('not a service-fabrik-operation, should return false', function () {
      queryParams.parameters = {
        foo: 'bar'
      };
      expect(utils.isServiceFabrikOperation(queryParams)).to.be.false;
    });
    it('service-fabrik-operation, should return true', function () {
      queryParams.parameters = {
        'service-fabrik-operation': 'token'
      };
      expect(utils.isServiceFabrikOperation(queryParams)).to.be.true;
    });
  });

  describe('#getRandomCronForOnceEveryXDaysWeekly', function () {
    const AssertionError = require('assert').AssertionError;
    it('should create a weekly random schedule - no options given', function () {
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* [0-6]', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly()
        )).to.be.eql(true);
    });
    it('should create a weekly schedule in the given range', function () {
      expect(RegExp(`[0-9]+ [0-9]+ \\* \\* [3-6]`, 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'start_after_weekday': 3,
          })
        )).to.be.eql(true);
    });
    it('should create a weekly schedule when interval given', function () {
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* 0,1,2,3,4,5,6', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'day_interval': 1,
          })
        )).to.be.eql(true);
    });
    it('should create a weekly schedule when interval and bounds are given', function () {
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* 3,4', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'start_after_weekday': 3,
            'start_before_weekday': 5,
            'day_interval': 1,
          })
        )).to.be.eql(true);
    });
    it('should support throw error if bounds are same', function () {
      expect(utils
        .getRandomCronForOnceEveryXDaysWeekly
        .bind(utils, {
          'start_after_weekday': 0,
          'start_before_weekday': 0,
          'day_interval': 1,
        })
      ).to.throw(AssertionError);
    });
    it('should create weekly cron within bounds', function () {
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* 0,1,2', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'start_after_weekday': 0,
            'start_before_weekday': 3,
            'day_interval': 1,
          })
        )).to.be.eql(true);
    });
    it('should create valid weekly cron when interval set to zero', function () {
      // interval 0
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* [0-6]', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'day_interval': 0,
          })
        )
      ).to.be.eql(true);
    });

    it('should create weekly cron when interval greter than 4, start day given', function () {
      // interval > 6, start_after_weekday provided
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* [3-6]', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'start_after_weekday': 3,
            'day_interval': 7,
          })
        )
      ).to.be.eql(true);
    });

    it('should create weekly cron when interval less than 4, start day given', function () {
      // interval > 6, start_after_weekday provided
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* 0,2,4,6', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'start_after_weekday': 0,
            'day_interval': 2,
          })
        )
      ).to.be.eql(true);
    });

    it('should create weekly cron when interal greter than weekdays', function () {
      // interval > 6
      expect(RegExp('[0-9]+ [0-9]+ \\* \\* [0-6]', 'g')
        .test(utils
          .getRandomCronForOnceEveryXDaysWeekly({
            'day_interval': 7,
          })
        )
      ).to.be.eql(true);

    });
  });

  describe('#getCronWithIntervalAndAfterXminute', function () {
    const AssertionError = require('assert').AssertionError;
    it('should support daily schedule', function () {
      expect(RegExp('[0-9]+ [0-9]+ \* \* \*').test(utils.getCronWithIntervalAndAfterXminute('daily'))).to.be.eql(true);
    });

    it('should support every \'24 hours\' schedule', function () {
      expect(RegExp('[0-9]+ [0-9]+ \* \* \*').test(utils.getCronWithIntervalAndAfterXminute('24 hours'))).to.be.eql(true);
    });

    it('should support every \'8 hours\' (divides 24) schedule', function () {
      expect(RegExp('[0-9]+ [0-9]+[\,]{1}[0-9]+[\,]{1}[0-9]+ \* \* \*').test(utils.getCronWithIntervalAndAfterXminute('8 hours'))).to.be.eql(true);
    });

    it('should support every \'9 hours\' (24 non divisible) schedule', function () {
      expect(RegExp('[0-9]+ [0-9\,]+ \* \* \*').test(utils.getCronWithIntervalAndAfterXminute('9 hours', 2))).to.be.eql(true);
    });

    it('should support every \'1 hours\' (24 non divisible) schedule', function () {
      expect(RegExp('[0-9]+ [0-9\,]+ \* \* \*').test(utils.getCronWithIntervalAndAfterXminute('1 hours'))).to.be.eql(true);
    });

    it('should not support invalid interval format', function () {
      expect(utils.getCronWithIntervalAndAfterXminute.bind(utils, 'random', 2)).to.throw(AssertionError);
    });

    it('should not support invalid schedule', function () {
      expect(utils.getCronWithIntervalAndAfterXminute.bind(utils, '35 hours')).to.throw(AssertionError);
    });
  });

  describe('#getBrokerAgentCredsFromManifest', function () {
    const manifest1 = {
      name: 'test-deployment-name',
      instance_groups: [{
        name: 'blueprint',
        jobs: [{
            name: 'blueprint',
            properties: {
              admin: {
                username: 'admin',
                password: 'admin'
              },
              mongodb: {
                service_agent: {
                  username: 'admin',
                  password: 'admin'
                }
              }
            }
          },
          {
            name: 'broker-agent',
            properties: {
              username: 'admin1',
              password: 'admin1',
              provider: {
                name: 'openstack'
              }
            }
          }
        ]
      }],
    };

    const manifest2 = {
      name: 'test-deployment-name',
      instance_groups: [{
          name: 'blueprint',
          jobs: [{
              name: 'blueprint',
              properties: {
                admin: {
                  username: 'admin',
                  password: 'admin'
                },
                mongodb: {
                  service_agent: {
                    username: 'admin',
                    password: 'admin'
                  }
                }
              }
            },
            {
              name: 'broker-agent',
              properties: {
                username: 'admin2',
                password: 'admin2',
                provider: {
                  name: 'openstack'
                }
              }
            }
          ]
        },
        {
          name: 'blueprint2',
          jobs: [{
            name: 'blueprint',
            properties: {
              admin: {
                username: 'admin',
                password: 'admin'
              },
              mongodb: {
                service_agent: {
                  username: 'admin',
                  password: 'admin'
                }
              }
            }
          }]
        }
      ]
    };

    const manifest3 = {
      name: 'test-deployment-name',
      instance_groups: [{
          name: 'blueprint2',
          jobs: [{
            name: 'blueprint',
            properties: {
              admin: {
                username: 'admin',
                password: 'admin'
              },
              mongodb: {
                service_agent: {
                  username: 'admin',
                  password: 'admin'
                }
              }
            }
          }]
        },
        {
          name: 'blueprint',
          jobs: [{
              name: 'blueprint',
              properties: {
                admin: {
                  username: 'admin',
                  password: 'admin'
                },
                mongodb: {
                  service_agent: {
                    username: 'admin',
                    password: 'admin'
                  }
                }
              }
            },
            {
              name: 'test-broker-agent',
              properties: {
                username: 'admin3',
                password: 'admin3',
                provider: {
                  name: 'openstack'
                }
              }
            }
          ]
        }
      ]
    };

    const manifest4 = {
      name: 'test-deployment-name',
      instance_groups: [{
        name: 'blueprint',
        jobs: [{
            name: 'blueprint',
            properties: {
              admin: {
                username: 'admin',
                password: 'admin'
              },
              mongodb: {
                service_agent: {
                  username: 'admin',
                  password: 'admin'
                }
              }
            }
          },
          {
            name: 'broker-agent-my',
            properties: {
              username: 'admin4',
              password: 'admin4',
              provider: {
                name: 'openstack'
              }
            }
          }
        ]
      }],
    };
    it('should return correct agent creds for all possible kind of manifest formats', function () {
      expect(utils.getBrokerAgentCredsFromManifest(manifest1)).to.eql({
        username: 'admin1',
        password: 'admin1'
      });
      expect(utils.getBrokerAgentCredsFromManifest(manifest2)).to.eql({
        username: 'admin2',
        password: 'admin2'
      });
      expect(utils.getBrokerAgentCredsFromManifest(manifest3)).to.eql({
        username: 'admin3',
        password: 'admin3'
      });
      expect(utils.getBrokerAgentCredsFromManifest(manifest4)).to.eql({
        username: 'admin4',
        password: 'admin4'
      });
    });
  });
});