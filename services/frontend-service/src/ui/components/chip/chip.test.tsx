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
import { Chip } from './chip';
import { render } from '@testing-library/react';
import { EnvPrio } from '../ReleaseDialog/ReleaseDialog';

describe('Chip', () => {
    const getNode = () => <Chip className={'chip--test'} label={'Test Me'} priority={EnvPrio.PROD} />;
    const getWrapper = () => render(getNode());
    it('renders a chip', () => {
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
            <span
              class="mdc-evolution-chip chip--test chip--test-prod"
              role="row"
            >
              <span
                class="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary"
                role="gridcell"
              >
                <span
                  class="mdc-evolution-chip__text-label"
                >
                  Test Me
                </span>
              </span>
            </span>
        `);
    });
});

const data = [
    {
        envPrio: EnvPrio.PROD,
        expectedClass: 'prod',
    },
    {
        envPrio: EnvPrio.PRE_PROD,
        expectedClass: 'pre_prod',
    },
    {
        envPrio: EnvPrio.UPSTREAM,
        expectedClass: 'upstream',
    },
    {
        envPrio: EnvPrio.OTHER,
        expectedClass: 'other',
    },
];

describe.each(data)(`Chip with envPrio Classname`, (testcase) => {
    it(`with envPrio=${testcase.envPrio}`, () => {
        const getNode = () => <Chip className={'chip--hello'} label={'Test Me'} priority={testcase.envPrio} />;
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.firstChild).toHaveClass(
            'mdc-evolution-chip chip--hello chip--hello-' + testcase.expectedClass
        );
    });
});
