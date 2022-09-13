/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { render } from '@testing-library/react';
import { ServiceLane } from './ServiceLane';
import { UpdateOverview } from '../../utils/store';
import { Spy } from 'spy4js';

const mock_ReleaseCard = Spy.mockReactComponents('../../components/ReleaseCard/ReleaseCard', 'ReleaseCard');

const sampleEnvs = {
    foo: {
        // third release card contains two environments
        name: 'foo',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    foo2: {
        // third release card contains two environments
        name: 'foo2',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    bar: {
        // second release card contains one environment, newest version
        name: 'bar',
        applications: {
            test2: {
                version: 3,
            },
        },
    },
    undeploy: {
        // first release card is for the undeploy one
        name: 'undeploy',
        applications: {
            test2: {
                version: -1,
            },
        },
    },
    other: {
        // no release card for different app
        name: 'other',
        applications: {
            test3: {
                version: 3,
            },
        },
    },
} as any;

describe('Service Lane', () => {
    const getNode = (overrides: { application: string }) => <ServiceLane {...overrides} />;
    const getWrapper = (overrides: { application: string }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        UpdateOverview.set({
            environments: sampleEnvs,
        });
        getWrapper({ application: 'test2' });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({ app: 'test2', version: -1 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({ app: 'test2', version: 3 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({ app: 'test2', version: 2 });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });
});
