'use strict';

const AddOns = require('../lib/bosh/manifest/AddOns');
const Networks = require('../lib/bosh/manifest/Networks');

describe('bosh', () => {
  describe('manifest', () => {
    describe('AddOns', () => {
      const networks = new Networks([{
        name: 'network1',
        type: 'manual',
        cloud_properties: {
          name: 'random'
        },
        subnets: [{
            az: 'z1',
            range: '127.0.0.1/26'
          }, {
            az: 'z2',
            range: '127.1.0.1/26'
          },
          {
            az: 'z3',
            range: '127.2.0.1/26'
          }
        ]
      }], 1, {
        size: 1
      });
      const context = {
        networks: networks.all
      };
      describe('#getAll', () => {
        it('returns list of addon jobs that are to be configured for the service', () => {
          let expectedJSON = [{
            name: 'iptables-manager',
            jobs: [{
              name: 'iptables-manager',
              release: 'service-fabrik',
              properties: {
                allow_ips_list: '127.0.0.2,127.1.0.2,127.2.0.2',
                block_ips_list: '127.0.0.1/26,127.1.0.1/26,127.2.0.1/26'
              }
            }]
          }];
          const addOns = new AddOns(context).getAll();
          expect(addOns).to.eql(expectedJSON);
        });
        it('throws error when requesting for an addon job thats not configured', () => {
          const addOns = new AddOns(context);
          expect(addOns.getAddOn.bind(addOns, 'iptables')).to.throw('Invalid add-on job type. iptables does not exist');
        });
      });
    });
  });
});