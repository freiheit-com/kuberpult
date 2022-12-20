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

describe('Chip', () => {
    const getNode = () => <Chip className={'chip--test'} label={'Test Me'} />;
    const getWrapper = () => render(getNode());
    it('renders a chip', () => {
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
            <span
              class="mdc-evolution-chip chip--test"
              role="row"
            >
              <span
                class="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary"
                role="gridcell"
              >
                <span
                  class="mdc-evolution-chip__ripple mdc-evolution-chip__ripple--primary"
                />
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
