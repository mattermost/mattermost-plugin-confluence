import React from 'react';
import PropTypes from 'prop-types';

import {Combobox} from '@mattermost/compass-ui/components/combobox';
import {ErrorMessage} from '@mattermost/compass-ui/components/error-message';
import {Select} from '@mattermost/compass-ui/components/select';
import {TextInput} from '@mattermost/compass-ui/components/text-input';

import Constants from '../constants';

import './confluence_field.css';

const REQUIRED_MESSAGE = 'This field is required.';

export default class ConfluenceField extends React.PureComponent {
    static propTypes = {
        required: PropTypes.bool.isRequired,
        value: PropTypes.oneOfType([
            PropTypes.string,
            PropTypes.number,
            PropTypes.object,
            PropTypes.array,
        ]),
        label: PropTypes.string.isRequired,
        onChange: PropTypes.func.isRequired,
        addValidation: PropTypes.func.isRequired,
        removeValidation: PropTypes.func.isRequired,
        fieldType: PropTypes.string.isRequired,
        readOnly: PropTypes.bool,
        type: PropTypes.string,
        placeholder: PropTypes.string,
        name: PropTypes.string,
        options: PropTypes.array,
        isMulti: PropTypes.bool,
        testId: PropTypes.string,
    };

    static defaultProps = {
        readOnly: false,
        options: [],
    };

    constructor(props) {
        super(props);
        this.state = {
            valid: true,
        };
    }

    componentDidMount() {
        if (this.props.addValidation) {
            this.props.addValidation(this.isValid);
        }
    }

    componentWillUnmount() {
        if (this.props.removeValidation) {
            this.props.removeValidation(this.isValid);
        }
    }

    handleChange = (value) => {
        if (!this.state.valid) {
            this.setState({
                valid: true,
            });
        }
        this.props.onChange(value);
    };

    handleSelectChange = (value) => {
        this.handleChange(this.props.options.find((option) => option.value === value) || null);
    };

    handleMultiChange = (values) => {
        this.handleChange(this.props.options.filter((option) => values.includes(option.value)));
    };

    isValid = () => {
        const {fieldType, value, required} = this.props;
        if (required &&
            (value === null ||
            (typeof value === 'string' && !value.trim()) ||
            (fieldType === 'dropDown' && value.length === 0) ||
            !value)
        ) {
            this.setState({
                valid: false,
            });
            return false;
        }
        return true;
    };

    renderLabel() {
        const {label, required} = this.props;
        if (!required) {
            return label;
        }

        return (
            <>
                {label}
                <span
                    className='confluence-field__required'
                    aria-hidden={true}
                >
                    {' *'}
                </span>
            </>
        );
    }

    renderField(invalid) {
        const {
            fieldType, value, type, placeholder, name, readOnly, options, isMulti, testId, required, label,
        } = this.props;

        if (fieldType === 'input') {
            return (
                <TextInput
                    id={testId}
                    label={this.renderLabel()}
                    type={type}
                    placeholder={placeholder}
                    value={value}
                    readOnly={readOnly}
                    required={required}
                    invalid={invalid}
                    onChange={this.handleChange}
                    data-testid={testId}
                />
            );
        }

        if (isMulti) {
            const selected = value || [];
            return (
                <Combobox
                    id={testId}
                    name={name}
                    label={this.renderLabel()}
                    listboxLabel={label}
                    options={options}
                    selectedOptions={selected}
                    value={selected.map((option) => option.value)}
                    multiple={true}
                    required={required}
                    invalid={invalid}
                    onChange={this.handleMultiChange}
                    submitOnEnter={false}
                    zIndex={Constants.Z_INDEX_MENU_PORTAL}
                />
            );
        }

        return (
            <Select
                id={testId}
                name={name}
                label={this.renderLabel()}
                listboxLabel={label}
                options={options}
                value={value ? value.value : ''}
                invalid={invalid}
                onChange={this.handleSelectChange}
                zIndex={Constants.Z_INDEX_MENU_PORTAL}
            />
        );
    }

    render() {
        const invalid = this.props.required && !this.state.valid;

        return (
            <div className='confluence-field'>
                {this.renderField(invalid)}
                {invalid && <ErrorMessage message={REQUIRED_MESSAGE}/>}
            </div>
        );
    }
}
