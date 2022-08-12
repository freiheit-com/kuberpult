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

import { Textfield } from './textfield';
import { render } from '@testing-library/react';

describe('Textfield', () => {
    it('renders correctly using Snapshot', () => {
        // given
        const { container } = render(<Textfield floatingLabel="Floating label" />);
        // when & then
        expect(container.firstChild).toMatchSnapshot();
    });

    test('renders correctly with leading icon', () => {
        // given
        const { container } = render(<Textfield leadingIcon="search" />);
        // when & then
        expect(container.querySelectorAll('div')[0]?.className).toEqual(
            'mdc-text-field mdc-text-field--outlined mdc-text-field--no-label mdc-text-field--with-leading-icon'
        );
        expect(container.querySelector('i')).toMatchInlineSnapshot(`
    <i
      class="material-icons mdc-text-field__icon mdc-text-field__icon--leading"
      role="button"
      tabindex="0"
    >
      search
    </i>
  `);
    });
});
