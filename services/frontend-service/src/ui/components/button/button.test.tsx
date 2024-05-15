/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import { Button } from './button';
import { render } from '@testing-library/react';

describe('Button', () => {
    const getNode = () => <Button highlightEffect={false} className={'button--test'} label={'Test Me'} />;
    const getWrapper = () => render(getNode());
    it('renders a button', () => {
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
    <button
      aria-label="Test Me"
      class="mdc-button button--test"
    >
      <div
        class="mdc-button__ripple"
      />
      <span
        class="mdc-button__label"
      >
        Test Me
      </span>
    </button>
  `);
    });
});
