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
import React from 'react';
import { render } from '@testing-library/react';
import { NavbarIndicator, NavListItem } from './navListItem';
import { MemoryRouter } from 'react-router-dom';

describe('Navigation List Item', () => {
    const getNode = (overrides?: {}, entries?: string[]): JSX.Element | any => {
        // given
        const defaultProps: any = {
            to: '/test',
            className: 'test-item',
        };
        return (
            <MemoryRouter initialEntries={entries}>
                <NavListItem {...defaultProps} {...overrides} />
            </MemoryRouter>
        );
    };
    const getWrapper = (overrides?: { to?: string; icon?: JSX.Element }, entries?: string[]) =>
        render(getNode(overrides, entries));

    it(`Renders a navigation item base`, () => {
        // when
        const { container } = getWrapper();
        // then
        expect(container.firstChild).toMatchSnapshot();
    });

    interface dataT {
        name: string;
        initialEntries: string[];
        to: string;
        expect: (container: HTMLElement) => void;
    }

    const data: dataT[] = [
        {
            name: 'Navigation Item Selected',
            initialEntries: ['/test'],
            to: 'test',
            expect: (container) =>
                expect(container.querySelectorAll('a')[0]?.className).toEqual(
                    'mdc-list-item mdc-list-item--activated test-item'
                ),
        },
        {
            name: 'Navigation Item Not Selected',
            initialEntries: ['/not-test'],
            to: 'test',
            expect: (container) =>
                expect(container.querySelectorAll('a')[0]?.className).toEqual('mdc-list-item test-item'),
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            const { initialEntries, to } = testcase;
            // when
            const { container } = getWrapper({ to: to }, initialEntries);
            // then
            testcase.expect(container);
        });
    });

    it(`Renders a navigation item with icon`, () => {
        // when
        const { container } = getWrapper({ icon: <svg>iconic</svg> });
        // when & then
        expect(container.querySelector('svg')).toMatchInlineSnapshot(`
    <svg>
      iconic
    </svg>
  `);
    });
});

describe('Display sidebar indicator', () => {
    interface dataT {
        name: string;
        pathname: string;
        to: string;
        expect: (container: HTMLElement, url?: string) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'Indicator is not displayed',
            pathname: '/test/',
            to: 'anotherTest',
            expect: (container) =>
                expect(container.querySelector(`.mdc-list-item-indicator--activated`)).not.toBeTruthy(),
        },
        {
            name: 'Indicator is displayed',
            pathname: '/test/',
            to: 'test',
            expect: (container) => expect(container.querySelector(`.mdc-list-item-indicator--activated`)).toBeTruthy(),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <NavbarIndicator {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { pathname: string; to: string }) => render(getNode(overrides));

    describe.each(data)(`Sidebar Indicator Cases`, (testcase) => {
        it(testcase.name, () => {
            const { pathname, to } = testcase;
            // when
            const { container } = getWrapper({ pathname: pathname, to: to });
            // then
            testcase.expect(container);
        });
    });
});
